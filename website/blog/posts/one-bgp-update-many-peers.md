---
title: One BGP UPDATE, many peers
date: 2026-08-04
author: Thomas Mangin
description: Why Ze's broader routing role needed a new forwarding design, and how I chose to reuse completed encoding work while keeping each peer's send independent.

deck: ExaBGP left routing decisions to another program. Ze's broader role gave me a new problem: a hundred independent peer decisions can still produce only two bodies worth rebuilding.

image: assets/blog/one-bgp-update-many-peers.svg
image-dark: assets/blog/one-bgp-update-many-peers-dark.svg
image-alt: One received BGP UPDATE passes through 100 independent peer decisions, producing two distinct encodings copied into separate peer-owned buffers.

---

I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp) so an ordinary process could speak BGP. It could announce a service or anycast prefix, inject a blackhole or FlowSpec rule, and turn received messages into text or JSON another program could use. I deliberately left the routing decision with the configuration or the process using its API.

That division of responsibility has been useful for many years. ExaBGP manages the sessions, while its user decides what to announce. Forwarding a route learned from one peer to other peers is a decision an external program can make, rather than an internal routing path which ExaBGP supplies for it.

Ze began with a migration path for those users: a compiled, multithreaded engine which could keep their configuration and process integrations. Its scope then grew to include a native RIB and policy, with route-server and route-reflection behaviour. I was taking on decisions which ExaBGP had deliberately left outside the program, and that required new design work.

The forwarding path in this article is one of those additions. One UPDATE can arrive through one peer and leave through a hundred others, each with its own policy and encoding requirements. If those hundred independent decisions produce only two different bodies, I want Ze to rebuild those two bodies without binding the hundred sends to a shared output lifetime.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude implemented the result comparison, generated AS path test cases and ran the competing benchmarks, as well as helping organise and draft the original text.*

## A new path can repeat old mistakes

For a route server, the loop which considers an UPDATE for every destination is the fan-out. Policy can suppress a route, replace its next hop or change its communities and AS path. Protocol rules and negotiated capabilities also affect the result, so each destination still needs its own decision even when many peers belong to the same group.

The straightforward implementation builds a body for every destination after that decision. In the hundred-peer example, two distinct results still cause a hundred builds. This is an illustration of repeated work, rather than a claim about a measured deployment, but it exposes the design problem without needing a large routing table.

Ze's first implementation repeated work within those builds as well. It revisited attributes, created temporary values which were then copied into output, and had separate attribute writers for queued and immediate announcements. The [fan-out architecture record](https://github.com/ze-software/ze/blob/main/docs/architecture/bgp/fanout-dedup.md) describes the timing of the old reuse check: it compared materialised wire objects, after the build which the comparison was supposed to save.

A compiled language makes repeated generation cheaper, but it still leaves the program generating the same answer repeatedly. I wanted to move the comparison earlier, to the point where Ze had finished deciding what a destination should receive but had not yet paid to build its body. The policy decision had to remain complete and independent, otherwise saving a rebuild could change which route a peer received.

## Finish the decision before building its bytes

The input to forwarding is an immutable wire representation of the UPDATE. Ze can read its path attributes and prefixes without duplicating the whole route into a second set of objects. For each destination, the [forwarding path](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_forward.go) records the changes required by protocol rules and export policy before it rebuilds the body.

That record may contain attribute replacements and removals, an AS path edit, or changes to the prefixes being announced or withdrawn. A policy decision can also suppress the route altogether. I need that distinction to survive the encoding stage: if a required change cannot be encoded, Ze must suppress the send to that destination rather than send the unchanged route which the policy was meant to alter.

```text
immutable source UPDATE requiring announcement edits
    -> complete protocol and policy edits for this peer
    -> compare the source identity and the complete edits
    -> reuse a matching rebuild, or encode a new body
    -> keep the rebuilt bytes in this peer's own buffer
    -> finish destination-specific framing and queue its send
```

The operation being shared builds the content after the fixed BGP header. Splitting and some encoding-context work still happen for each destination afterwards. In the hundred-peer example, two successful reusable rebuilds can replace a hundred rebuilds, but every peer keeps its own send and its remaining destination-specific work.

Delaying the build is often called delayed materialisation. For me, the important part is where the delay ends. Comparing half a policy decision would only say that two peers agree so far, and an edit recorded later could invalidate the reuse. Ze has to compare the complete input to the particular operation it intends to share.

## Equality must include everything the rebuild reads

Ze writes the completed edits in a stable byte representation called a [digest](https://github.com/ze-software/ze/blob/main/internal/component/bgp/filterapi/fingerprint.go). Variable-length values carry their lengths, and operations retain the order used by the rebuild. Without those boundaries, the same sequence of bytes could describe different instructions.

Leaving the advertised prefixes alone is different from replacing them with an empty list. Both might look like an empty byte slice if the comparison discarded presence information, yet one preserves the announcements and the other removes them. The digest retains that difference.

An AS path generator introduces another form of the same problem. Ze reuses the generator object between peers, so its address can stay the same while the path it describes changes. The digest includes the bytes the generator would produce. Comparing the object address would compare reusable working storage with itself, without establishing anything about the next answer.

The [reuse table](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup.go) also requires the same immutable source UPDATE object. Equal edits applied to different source bytes can produce different results, and export policy can supply a replacement source for one destination. Two separate source objects containing equal bytes currently rebuild separately. I accept that missed reuse, because assuming equality in the other direction could substitute one route for another.

The digest can be longer than I want to compare against every recorded result, so Ze first calculates a 64-bit fingerprint to find likely matches. Every candidate still has its complete digest compared byte for byte. Two different digests can have the same fingerprint, and that collision adds comparison work and increments a counter. It cannot authorise reuse.

I chose that separation because an unlucky hash must never become a routing decision. It also lets the fingerprint remain cheap: the current mixing function processes eight bytes per round. The first version used FNV-1a, with a multiply for every byte dependent on the previous result. A [48-byte digest was recorded at around 35 ns](https://github.com/ze-software/ze/blob/main/internal/component/bgp/filterapi/fingerprint.go) in that earlier implementation, paid for every destination whether reuse succeeded or failed. Once full equality carries correctness, spending more on the hash needs a performance reason of its own.

The first destination with a new result causes a rebuild, and later destinations with the same source and digest can copy that result. The table lasts for one forwarding call and records at most 128 classes with up to 64 KiB of retained digests. An individual digest is limited to 2 KiB.

Those limits follow from the source of the input. Attribute values can come from the network, and retaining or hashing unbounded descriptions for every destination could cost more than the builds being avoided. An edit set which cannot be represented within the digest limit bypasses reuse. When the table cannot record a new class, Ze counts that refusal and keeps the peer's independently built result, while existing entries remain usable.

## Build the result once, into its destination

Moving the comparison before encoding only helps the matching destinations. A new result still needs a builder, and I wanted to remove the temporary copies from that path too.

The [body builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build.go) indexes the original attributes and plans which regions to retain, replace or remove. It walks that plan to calculate the exact output length, acquires storage and walks it again to write. If communities remain unchanged while another attribute changes, their original bytes go directly into the destination buffer. Retained runs within an edited community list can also be copied from the source without first assembling a temporary copy of the complete list.

Exact sizing replaced a guessed amount of spare capacity. It also gives the builder a refusal point before writing when the requested result cannot fit the supported body size. I wanted an encoding failure to remain attached to the modification which failed, rather than become an excuse to send the source unchanged.

The [configured and queued announcement writer](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_build.go) now uses the same attribute-emission machinery. The earlier separate writers made scheduling another place where encoding rules could diverge. Sharing that machinery means the route's place in a queue does not select a separate rule for where an attribute belongs.

An unchanged UPDATE is a separate case. When the encoding contexts match and no path-identifier rewrite or split is needed, [the forwarding body builder already borrows the original body](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go) for the send. An earlier version of this article described this as future work, which was wrong. The source remains immutable, and the receive cache provides its lifetime. My choice to copy equivalent rebuilt output between peers does not remove the eligible unchanged path's borrowing.

## The copy I chose to keep

The first design considered sharing one rebuilt buffer among all equivalent destinations. That would remove the last copy, but each destination sends at its own pace. The buffer could return to storage only after every referencing send had finished, including the slowest one.

I preferred the existing rule that one rebuilt send owns one output buffer, unless the measurement justified taking on that shared lifetime. Claude ran the comparison before the design acquired that extra coordination. In the [original published measurements](https://github.com/ze-software/ze/blob/93faff0744e0801c2d22cf17060bc5496b66863c/website/blog/posts/one-bgp-update-many-peers.md), rebuilding took 426.85 ns and copying the result took 2.07 ns, about half of one per cent of the rebuild time.

The size of that copy is important to the decision. [BenchmarkFanoutRebuildOnly](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_bench_test.go) uses a [fixture](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_test.go) announcing `10.0.0.0/24` and `10.0.1.0/24`, with ORIGIN, a three-AS path, NEXT_HOP, MED, LOCAL_PREF and eight communities. Its body is 89 bytes, excluding the BGP header, and changing the next hop to `10.99.0.1` leaves that length unchanged. The fixture has neither MP_REACH nor attribute 40, so the other recorded edits have nothing to change.

These are historical measurements of that small payload. The rebuild arm has no peer output pool and includes its owned-result fallback, while the copy arm copies between already allocated slices. Buffer acquisition, policy and TCP are outside the copy timing. The ratio helped me decide how much complexity this copy justified, but it cannot predict the saving for arbitrary UPDATEs.

Ze pays a copy for each later destination which reuses a rebuild. The loop collects pending sends and dispatches them after the comparisons have finished, so a buffer cannot be returned while the reuse table still uses it as a source. Afterwards, every send can finish and release its own output independently. I kept a known cost in exchange for avoiding coordination between otherwise independent rebuilt sends.

Slow queues still need their own lifetime rule. Items moved into overflow take owned copies, including bodies which could otherwise borrow the unchanged source, because they may wait beyond the receive cache's retention safety valve. [How Ze manages memory](../how-ze-manages-memory/) follows that boundary and the pool fallbacks. Separate rebuilt output buffers solve one ownership problem, and borrowed input still needs its owner accounted for.

## A configured route starts with different inputs

Configured and API announcements are closer to ExaBGP's usual workload. They have no received body to edit, so comparing a source UPDATE and an edit digest would answer the wrong question. Ze already has the route it wants to announce and needs to know which destinations require the same build.

The configured path groups established destinations using [the same per-peer facts passed to the batch builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch.go). The resolved next hop and session role are among those facts, along with the encoding and message-grouping settings. Within a batch, equal facts permit a shared build when grouping is enabled.

I wanted the builder's inputs and the grouping key to remain the same description. Keeping a second list solely for the key would require every later change to remember both lists, and a missing field could put peers needing different bytes in one group. The current structure makes a new per-peer build input part of the comparison too.

The two paths therefore establish reuse differently. Received-route reuse compares the immutable base and completed edits, while a configured announcement compares its build inputs. A peer group or a matching OPEN exchange can supply some of that information, but neither is enough on its own to prove that all later decisions will agree.

## Measure the path the design changes

The [fan-out benchmark](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_bench_test.go) uses the same route fixture, with per-peer next-hop policy producing the requested number of distinct results. It compares the forwarding path with edit-set reuse enabled and disabled in one harness, after an untimed warm-up. The harness exercises forwarding and pool dispatch, without a live TCP connection or the network receive loop.

That scope is deliberate. A measurement dominated by another part of the router would not tell me whether comparing these decisions was worth doing. The benchmark also counts body rebuilds, so the timing can be read alongside the operation the change was meant to remove.

The original article reported these Apple M4 Max results, with six runs per case in alternating order and the median time per destination. They are retained historical measurements, rather than a new benchmark of the current tree.

| Destinations | Distinct results | Reuse disabled | Reuse enabled | Change |
|---:|---:|---:|---:|---:|
| 2 | 1 | 1,228 ns | 1,090 ns | -11.2% |
| 2 | 2 | 1,233 ns | 1,286 ns | +4.3% |
| 10 | 2 | 1,234 ns | 989 ns | -19.8% |
| 100 | 2 | 1,100 ns | 729 ns | -33.7% |
| 100 | 100 | 1,102 ns | 1,115 ns | +1.2% |

With reuse disabled, the harness rebuilds once per destination. With it enabled, the intended count is once per distinct result. Allocations across the wider harness remained the same in the reported runs because the stages around the rebuild still allocate. The separate [warm rebuild checks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go) cover the encoder when output storage is available.

The cases where every peer needs different bytes expose the cost I am accepting: recording and comparing results added 1.2 to 4.3 per cent in these runs. Repeated answers paid that comparison back, with the hundred-destination, two-result case reducing the measured time per destination by 33.7 per cent. Neither result establishes TCP throughput, route convergence or complete-router performance. The [architecture record](https://github.com/ze-software/ze/blob/main/docs/architecture/bgp/fanout-dedup.md) contains an earlier measurement set with different timings and percentages, rather than the raw record for this table.

## Where I stopped

The encoder still walks the plan once to size it and again to write it. Some lengths are already known while edits are recorded, so carrying them forward may remove part of that repeated calculation. For now, both operations use the same emission walk. I prefer keeping their agreement visible until there is evidence that another arrangement is worth maintaining.

Changing the source buffer itself would require a stronger ownership argument. Every later policy decision and reader would have to be finished with the original bytes, the buffer would need room for the change, and its lifetime would have to pass safely to the sends. Keeping the input immutable avoids those conditions. Unchanged eligible sends already borrow it, and modified sends can reuse a completed rebuild without acquiring permission to alter the source.

This is also where my experience and Claude's implementation have different jobs. I decide which work may be reused and which failure must prevent a route from leaving. Claude wrote the comparison, generated AS path cases and ran the competing benchmarks. That legwork made the design much more practical for a solo developer, but the measurements still had to answer a decision about routing correctness and ownership.

ExaBGP's deliberately narrow role let another program own the routing decision. Ze's wider role puts more of that responsibility inside the engine, and preserving its programmable interface does not make the new forwarding path a port. I can carry forward the experience of operating ExaBGP while accepting that a hundred independent destinations need a design ExaBGP never had to provide.

*Last updated: 9 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/one-bgp-update-many-peers.md).*
