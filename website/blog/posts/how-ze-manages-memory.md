---
category: platform
title: How Ze reuses memory for BGP UPDATEs
date: 2026-08-04
author: Thomas Mangin
description: From the Amiga demo scene to BGP UPDATEs, earlier experience informs how Ze reuses memory and avoids unnecessary allocation in Go.

deck: Memory efficiency starts with how you design the code.

image: assets/blog/how-ze-manages-memory.svg
image-dark: assets/blog/how-ze-manages-memory-dark.svg
image-alt: Readers share a received BGP UPDATE while it remains in the cache. Changed messages and routes kept in the RIB use separate storage.

---

Rust, Zig or C may come to mind first when you think about control over memory. In Ze, I use Go, but bring the same attention to where memory is allocated and how it is reused.

An unnecessary allocation looks harmless until a router repeats it for every route. When designing Ze, I brought some older programming habits to a language with a garbage collector.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude helped with implementation, tests and measurements, and with organising and drafting the original text.*

## From the Amiga to Ze

As a young and very inexperienced programmer, I learned assembly and low-level programming on the Amiga. I followed the demo scene, where programmers produced remarkable results within very limited memory and processing power. Those experiences still shape how I look at software.

When I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp), I wanted an ordinary program to be able to announce routes without implementing BGP itself. I also wanted the code to be easy to read and understand, so that other people could contribute and add support for more RFCs. I valued that accessibility more than performance. ExaBGP was never designed to be a particularly memory-efficient or CPU-efficient implementation. Ze, however, is.

To my mind, too many newer RFCs give too little attention to efficient processing or compact protocol layouts. Modern machines make the constraints I grew up with much less pressing for everyday software development, but the knowledge still informs my design decisions. Knowing assembly is useful even when I am writing Go.

With Ze, I wanted efficiency to be part of the design from the start. Go has a garbage collector, but we can write the repeated processing much as we would in C, allocating working storage ahead of time and managing its reuse ourselves. Pools let us keep buffers available and decide when they can hold new data, instead of creating and discarding an object for every operation. Avoiding those allocations can substantially [reduce the work left to the garbage collector](https://go.dev/doc/gc-guide#Eliminating_heap_allocations), although the runtime still manages the underlying memory.

Before adding an object or copying a message, we should be able to explain why that storage is necessary. Following a BGP UPDATE through Ze makes those decisions concrete, because an unnecessary operation here can be repeated for every route.

## Use the data we already have

A BGP UPDATE arrives as a sequence of bytes containing route announcements or withdrawals, together with attributes such as the next hop and AS path. Ze receives those bytes in a buffer, an area of memory used to hold them. One way to process the UPDATE would be to decode every attribute into a Go object, work with those objects, then encode them again when sending the route onwards. If the code only needs the next hop, decoding every attribute wastes CPU cycles and may also allocate memory for data it never uses.

Ze's [WireUpdate](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/wire_update.go) keeps the received bytes and provides access to the attributes within them. Its attribute index records where values occur, and a caller can read the next hop without constructing decoded objects for every other attribute. Several readers can also share the same stored message while it remains available. The input still has to be validated, so the saving comes from avoiding representations and copies which that processing does not require.

Sharing stops being suitable when one reader needs different bytes. If policy changes an attribute before an UPDATE is sent, the other readers must still see the original message. Ze's [buffer-first approach](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md) lets the encoder write the changed message directly into output storage supplied by its caller. The necessary new representation then goes into its destination without an intermediate message that would immediately be copied again.

## A fixed size makes reuse straightforward

The traditional maximum BGP message size is [4,096 bytes](https://www.rfc-editor.org/rfc/rfc4271#section-4), which is also a common [memory-page size](https://docs.kernel.org/mm/page_tables.html). A page is a unit in which the operating system maps memory. An implementation handling messages within that limit can allocate a pool of fixed-size buffers, then cycle through them as messages arrive and processing finishes. Once enough buffers are available, receiving the next message need not require another buffer allocation.

The size limit has since been extended: [RFC 8654](https://www.rfc-editor.org/rfc/rfc8654.html#section-4) allows messages up to 65,535 bytes when the receiver has advertised support, with exceptions for OPEN and KEEPALIVE. A pool must accommodate the messages a session can receive. The useful principle survives the change in limit: reserve suitable storage and reuse it once its readers have finished.

Ze's [mixed-size overflow pool, `MixedBufMux`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/bufmux.go), uses 64 KiB blocks that can serve one extended message or sixteen 4 KiB messages. Once all the borrowed slices are returned, the block can be used in either form again. The memory can therefore follow the mix of small and large messages without being tied to separate pools for each size.

## Reuse needs a clear stopping point

After the readers have finished with an UPDATE, its storage can hold another message. Keeping available buffers in a pool avoids allocating fresh storage for each arrival, but introduces a release decision: the code returning a buffer has to respect every reader's permitted lifetime.

Garbage collection does not make this reuse safe. Returning a buffer to a pool leaves its underlying memory allocated, and a slice, a view of the bytes in that memory, can still refer to it. If the next message overwrites the buffer before that reader has finished, code expecting the old route will read a different message. The memory remains accessible throughout; its contents have changed too soon.

Ze therefore has explicit [rules for how long readers can use shared bytes](https://github.com/ze-software/ze/blob/main/docs/architecture/memory/lifetime-contracts.md). An event handler can inspect a received UPDATE during delivery, but keeping it afterwards requires a snapshot in independent storage. Taking that copy before returning lets later processing continue after the original buffer has been reused. Avoiding every copy would defeat the point if a retained route could change underneath its reader.

A pool occupies memory even while its buffers sit unused, so fewer allocations can come at the cost of keeping more memory between bursts. Ze's [shared buffer pools](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/bufmux.go) support a budget for growth and release surplus blocks when they have been returned and the remaining blocks have room. Those limits apply to the pools concerned, without bounding the daemon's total memory use, and growing storage still requires allocations.

## Store repeated route attributes once

The routing information base, or RIB, keeps routes long after their receive buffers have been reused, so it needs independent storage. Keeping a copy of the whole packet would also keep bytes the RIB no longer uses. Giving every route its own attribute values avoids that waste but introduces another: many of those values are identical across the routing table.

Routes often share an AS path or the same communities, the values operators attach to routes for policy. Ze's [RIB storage](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go) stores common attributes individually, using an [attribute pool](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/pool.go) to keep equal values together. The first occurrence is copied into independent storage, and later routes hold references to the existing value for as long as they use it. This technique is called interning.

Sharing individual attributes lets two routes with different preferences still share the same AS path. Changing one value does not require another copy of all the others. Here, the saving concerns memory retained for the life of a route, as well as the temporary allocations avoided during message processing.

## Apply the same approach to text

Many of these patterns will be familiar to anyone who has read code by Daniel J. Bernstein, usually known as DJB. Earlier in my career we used his mail server, [qmail](https://cr.yp.to/qmail.html), and I worked on it for a long time, writing many patches. His approach to building text in reusable storage influenced how I write software.

DJB's [`stralloc` interface](https://cr.yp.to/lib/stralloc.html) keeps track of a string's length and available capacity, so more text can be appended without another allocation while space remains. Go's byte slices remind me a lot of `stralloc`, with their explicit length and capacity.

Go's [`fmt` package](https://pkg.go.dev/fmt) follows the C library's familiar `printf` and `sprintf` style with `Printf` and `Sprintf`. The standard library also provides [`strings.Builder`](https://pkg.go.dev/strings#Builder), which assembles a string from successive writes while minimising copying.

This matters when Ze produces text for an application, the job ExaBGP began with. Converting each route value into a string before joining the strings can create temporary storage even if the binary processing reused its buffers. An address followed by a separator and a number can instead be written into consecutive bytes of one buffer, without first turning the number into a separate string. Once the result has been consumed, the same buffer can be reset and used for the next value.

Ze's [`textbuf.Buffer`](https://github.com/ze-software/ze/blob/main/internal/core/textbuf/textbuf.go) follows that builder pattern, with typed methods for appending values such as addresses and integers, and includes 128 bytes of storage. A local buffer initialised with `Reset` can build short text without a separate heap allocation. A caller that consumes the completed bytes before resetting the buffer can read them directly, without copying the result.

If the text has to survive reuse of the buffer, the caller requests an independent string. The implementation copies bytes held in the buffer's built-in array, or transfers the storage if the bytes are already held in a separate allocated array. Either way, the string remains available and the buffer is emptied. The [text buffer design](https://github.com/ze-software/ze/blob/main/docs/architecture/textbuf-string-building.md) explains how the choice follows from how long the result will be kept, just as it does for an UPDATE.

## Recomputing can cost less than fetching

The relative cost of computation and memory access has changed over the years. A calculation that once justified keeping its result in memory can now be cheaper to repeat than to fetch from main memory. For a value that takes only a few instructions to regenerate from data already at hand, looking up a saved copy can cost more than doing the work again.

Keeping a result in a program's cache does not guarantee that it is in the CPU's cache. The CPU keeps recently used data in small, fast caches, with L1 closest to the executing instructions, followed by larger but slower levels such as L2 and L3. When a loop's working data stays in L1, repeated accesses can be many times faster than fetching it from main memory. Keeping more calculated results can enlarge that working data enough to lose the advantage.

The table combines illustrative timing examples with cache capacities for the base [M1 from 2020](https://www.apple.com/newsroom/2020/11/apple-unleashes-m1/) and [M6 from 2026](https://www.apple.com/newsroom/2026/08/apple-introduces-m6-and-m5-ultra-for-a-big-leap-in-performance-and-ai-compute/). The times are not measurements of those Apple chips or of Ze, and the instruction example assumes a 3 GHz CPU.

L1 capacities cover data storage per core and vary by core type; instruction caches are separate. L2 capacities are shared within the listed clusters, while the system-level cache (SLC) is shared across the chip and listed separately from CPU L3. P means performance core, E efficiency core, and S Apple's Super core.

| Operation or storage level | Illustrative latency | Relative to a 1 ns L1 hit | M1 (2020) cache capacity | M6 (2026) cache capacity |
|---|---:|---:|---|---|
| Simple integer addition, operands in registers | 0.33 ns (one cycle at 3 GHz) | 0.33× | — | — |
| L1 data cache hit | 1 ns | 1× | 64–128 KiB per core | 64–128 KiB per core |
| L2 cache hit | 4 ns | 4× | 12 MiB across 4 P-cores; 4 MiB across 4 E-cores | 20 MiB across 2 S-cores and 4 P-cores; 8 MiB across 6 E-cores |
| CPU L3 cache hit (Intel example) | 40 ns | 40× | Different layout; SLC below | Different layout; SLC below |
| System-level cache (SLC) | Not estimated here | — | 8 MiB shared | About 16 MiB shared |
| Main memory (DRAM) access | 80 ns | 80× | — | — |
| NVMe SSD, random 4 KiB read | 70 µs (70,000 ns) | 70,000× | — | — |
| Hard disk, random positioning before a read | About 12.5 ms (12,500,000 ns) | 12,500,000× | — | — |

The timing examples use [Intel's instruction tables](https://cdrdv2-public.intel.com/821614/356477-Software-Optimization-Manual_V2.pdf#page=150) and [2016 memory hierarchy](https://www.intel.com/content/www/us/en/developer/articles/technical/memory-performance-in-a-nutshell.html), the [Micron 7500 specification](https://sg.micron.com/content/dam/micron/global/public/products/data-sheet/ssd/7500-ssd-tech-prod-spec.pdf) (median random 4 KiB read at queue depth one), and the [Seagate Surveillance HDD manual](https://www.seagate.com/www-content/product-content/sv35-fam/surveillance-hdd/en-us/docs/100762250k.pdf) (8.5 ms typical read seek plus 4 ms rotational delay). Actual latencies vary with hardware and access pattern; overlapping work means these ratios do not predict whole-program speed.

The capacities draw on [M1 CPU-cache research](https://www.sra.uni-hannover.de/Publications/2024/wrenger_24_jsa.pdf) and [SLC analysis](https://chipsandcheese.com/p/igpu-cache-setups-compared-including-m1), and [M6 hardware observations](https://arstechnica.com/gadgets/2026/09/apple-m6-mac-mini-review-300-price-hike-spoils-a-nice-upgrade/) and [memory-hierarchy measurements](https://arxiv.org/html/2609.32237v1#S4).

Shared caches have grown, but L1 data capacity remains around a hundred kilobytes per core, with other data competing for that space. Caching remains useful when it avoids expensive computation, and regenerating a value still requires access to its inputs. The choice depends on how much work the calculation takes and where the data is when we need it. In Ze, avoiding unnecessary representations helps keep the data used together compact, as well as reducing allocations.

## Check that the intended saving exists

Choosing a design that avoids temporary storage does not establish how much memory its implementation allocates. Go's compiler can keep some values in a function's local storage while allocating others on the heap, and the result can change with the compiler. We therefore need to measure the compiled operations, even when the code appears to avoid allocation.

Go has excellent [profiling and diagnostic tools](https://go.dev/doc/diagnostics#profiling). With `pprof`, CPU profiles show where the program spends its CPU time, while [heap and allocation profiles](https://pkg.go.dev/runtime/pprof#hdr-Heap_profile) distinguish memory still in use from the total allocated over time. The reports connect those costs to functions and source lines, and can be read as text as well as explored visually.

This tooling makes AI-assisted optimisation particularly effective for me. An agent can follow an expensive call back into the code and try a focused change, then run the same workload again to compare CPU time and allocations. It helps the agents optimise Ze and gives me evidence to assess their changes, including whether the saving is worth any added complexity.

Ze's allocation assertions exercise repeated operations once their working storage is available. The [UPDATE rebuild tests](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go), for example, provide an available output buffer and require peer-specific attribute changes to allocate no heap memory. The community-removal test separately checks which values remain and their order. Both checks are necessary, because reducing allocations would be useless if rebuilding the message changed its meaning.

These checks cover the named operations; session creation, pool growth and other forwarding work still allocate memory.

The original measurements published with this article used Go 1.26.5 on an Apple M4 Max running macOS on arm64, with allocation reporting and medians of five runs. They have not been rerun for this revision.

The examples include BFD, the Bidirectional Forwarding Detection protocol, where Ze applies the same buffer reuse approach to control packets. The attribute measurements cover sharing a value already in the pool and reading it through a handle, the reference used to retrieve that stored value. Both operations reuse existing storage.

| Operation | Median time | Heap bytes | Heap allocations |
|---|---:|---:|---:|
| BFD packet encode and parse using a pool | 31.7 ns | 0 | 0 |
| Intern an existing attribute value | 145 ns | 0 | 0 |
| Read a stored attribute through its handle | 6.65 ns | 0 | 0 |

The [BFD benchmark](https://github.com/ze-software/ze/blob/21051d57f2/internal/component/bfd/packet/bench_test.go) acquires a packet buffer, writes and parses one unauthenticated control packet, then releases it. The [attribute-pool benchmarks](https://github.com/ze-software/ze/blob/21051d57f2/internal/component/bgp/attrpool/benchmark_test.go) use the existing value `benchmark-data` for interning and handle lookup. These links preserve the benchmark source available with the early article.

These microbenchmarks do not measure storing a complete new route, TCP throughput or route convergence, and their timings are not performance targets for a different compiler or machine.

A useful way to apply this elsewhere is to follow one piece of data from input to final use, recording its representation and how long each reader needs it. Each copy should have a reason why the existing bytes cannot serve the next operation. A different representation or an independent lifetime can justify new storage, while transferring ownership of existing storage may require no copy at all.

Reuse adds development work. We have to define who owns the storage and when a reader must stop using it, then check that the intended saving exists without changing the result. For a rarely used operation, that bookkeeping can cost more effort than the saving justifies. On a path repeated for each route, removing an unnecessary copy removes the same cost from every repetition.

## Addendum: Goose and memory management

Since this article was first written in August 2026, Wouter van Oortmerssen's [Goose](https://github.com/aardappel/goose) programming language has become publicly available, with [compiler releases in October](https://github.com/aardappel/goose/releases/tag/v2026.1). Its approach to memory management is particularly interesting in the context of the decisions above.

Goose's [memory model](https://github.com/aardappel/goose/blob/master/docs/goose_spec.md#12-memory-model) places dynamic values on data stacks assigned by the compiler, rather than allocating individual objects on a general-purpose heap. Growing a value advances a pointer through reserved address space, and leaving its owning scope makes the storage reusable by moving that pointer back. The compiler checks that references cannot outlive their owners. Compact layouts also let nested data sit together, avoiding a separate allocation for each part.

There is a connection with arenas in [Zig](https://ziglang.org/documentation/master/#Choosing-an-Allocator) or [Odin](https://odin-lang.org/docs/overview/#allocators), where a programmer groups allocations that can be released together, for example at the end of a request. Goose makes scope-based storage and its lifetime checks part of the language itself. I find that worth following: the relationship between data layout and lifetime directly shapes how programs are written.

*Originally written: 4 August 2026. Last updated: 13 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/how-ze-manages-memory.md).*
