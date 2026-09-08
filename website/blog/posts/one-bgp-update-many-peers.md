---
title: One BGP UPDATE, many peers
date: 2026-08-04
author: Thomas Mangin
description: How Ze finishes each peer's policy decision before reusing a matching UPDATE-body rebuild, with separate ownership for each rebuilt send.

deck: A hundred independent peer decisions can produce two distinct bodies. Ze compares the completed changes before rebuilding, then copies matching output into each peer's own buffer.

image: assets/blog/one-bgp-update-many-peers.svg
image-dark: assets/blog/one-bgp-update-many-peers-dark.svg
image-alt: One received BGP UPDATE passes through 100 independent peer decisions, producing two distinct encodings copied into separate peer-owned buffers.

---

A route server can receive one BGP UPDATE and consider it for a hundred peers. Each destination needs its own policy decision, but those hundred decisions may produce only two different messages. Rebuilding the same body for every destination repeats work whose answer is already known.

[ExaBGP](https://github.com/Exa-Networks/exabgp) leaves routing decisions to its configuration and the process using its API. I wrote it so an ordinary program could announce routes and consume BGP events. Ze's RIB and policy support give it a broader forwarding role, where a route learned from one peer can leave through many others. I wanted Ze to reuse the repeated encoding without tying those peers' sends to one shared output buffer.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude implemented the result comparison, generated AS path test cases and ran the competing benchmarks, as well as helping organise and draft the original text.*

## Finish the decision before building its bytes

The loop which considers an UPDATE for every destination is the fan-out. Policies may suppress the route or change its attributes, such as the next hop or communities. Protocol rules and negotiated capabilities can also affect what each peer receives, so membership of one peer group is insufficient evidence that the output will match.

Ze's [forwarding path](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_forward.go) records the changes required for each destination before rebuilding its UPDATE body. That record can contain attribute replacements and removals, AS path edits, or changes to the prefixes being announced or withdrawn. A decision to suppress the route produces no send, and a required modification which cannot be encoded also prevents that destination's send. Falling back to the unchanged route would defeat the policy which asked for the change.

For a modified announcement, the sequence is:

```text
immutable source UPDATE
    -> complete protocol and policy edits for this peer
    -> compare the source identity and the complete edits
    -> reuse a matching rebuild, or encode a new body
    -> keep the rebuilt bytes in this peer's own buffer
    -> finish destination-specific framing and queue its send
```

The reusable body is the content after the fixed BGP header. Splitting and some encoding-context work still happen per destination afterwards. The optimisation therefore saves a body rebuild; it does not collapse a hundred peers into two sends, or promise that every later encoding operation has disappeared.

## Equality includes the source

Ze writes the completed edits in a stable byte representation called a [digest](https://github.com/ze-software/ze/blob/main/internal/component/bgp/filterapi/fingerprint.go). Variable-length values carry their lengths, and operations retain the order used by the rebuild. An instruction to leave the advertised prefixes alone must remain different from an instruction to replace them with an empty list, even though both might otherwise look like an empty byte slice.

An AS path generator contributes the bytes it would produce. Comparing its address would be wrong because Ze reuses the same generator object between peers, and it can describe a different path on the next iteration. The comparison has to cover the answer the encoder will read.

The [reuse table](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup.go) also requires the same immutable source UPDATE object. Equal edits applied to different input bytes can produce different output, for example when export policy supplies a replacement source message for one destination. Two separate source objects containing equal bytes currently rebuild separately. That misses an opportunity to reuse, but it cannot substitute one route for another.

Ze calculates a short 64-bit fingerprint of the digest to find likely matches, then compares each candidate's complete digest byte for byte. Two different digests can have the same fingerprint. A collision adds comparison work and increments a counter; it never permits reuse by itself. The fingerprint can consequently use a cheap mixing function, with eight bytes processed per round, because correctness rests on the full comparison.

The first matching class is built once and later peers copy those bytes into their own output buffers. In the hundred-peer example, two distinct edit sets over the same source require two rebuilt bodies, provided they fit within the reuse limits. Each peer still has its independent decision and send.

The table exists for one forwarding call and records at most 128 classes with up to 64 KiB of retained digests. An individual digest is limited to 2 KiB. Network-controlled attribute values make an unbounded table or unbounded per-peer hashing a poor trade, even when it would save a rebuild. A digest which cannot be represented within the limit bypasses reuse; a full table counts the class it cannot record and the peer keeps its independently built result. Existing entries remain usable.

## Encode into the destination

On a miss, [the body builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build.go) indexes the original attributes and plans which regions to retain, replace or remove. It walks that plan to calculate the exact output length, acquires storage and walks it again to write. Unchanged communities can go straight from the input into the destination buffer while another attribute is replaced, without first copying the whole list into temporary storage.

Exact sizing replaced a guessed amount of spare capacity. It also gives the encoder a refusal point before writing when a requested result cannot fit the supported body size. The [configured and queued announcement writer](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_build.go) uses the same attribute-emission machinery, so scheduling does not select a different rule for placing attributes.

An UPDATE which needs no changes is a separate case. When the encoding contexts match and no path-identifier rewrite or split is needed, [the forwarding body builder already borrows the original body](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go) for the send. The input remains immutable. An earlier version of this article described that optimisation as future work, which was wrong: the choice to copy applies to sharing rebuilt output between peers, while the receive cache already supplies the lifetime for eligible unchanged sends.

## The copy we chose to keep

Sharing one rebuilt buffer would remove the last copy, but each destination sends at its own pace. Ze would need to keep that buffer until every referencing send had finished, including the slowest one. I preferred to retain the existing rule that one rebuilt send owns one output buffer unless the measurement justified changing it.

The comparison comes from [BenchmarkFanoutRebuildOnly](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_bench_test.go). Its [payload fixture](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_test.go) announces `10.0.0.0/24` and `10.0.1.0/24`, with ORIGIN, a three-AS path, NEXT_HOP, MED, LOCAL_PREF and eight communities. The UPDATE body is 89 bytes, excluding the BGP header. The benchmark changes the next hop to `10.99.0.1`; its MP_REACH next-hop edit and attribute-40 suppression have nothing to change in this fixture, and the rebuilt body remains 89 bytes.

The [original published measurements](https://github.com/ze-software/ze/blob/93faff0744e0801c2d22cf17060bc5496b66863c/website/blog/posts/one-bgp-update-many-peers.md) were 426.85 ns for rebuilding and 2.07 ns for copying the result, about half of one per cent of the rebuild time. These are historical measurements of that small payload. The rebuild arm calls the builder without a peer output pool and includes its owned-result fallback, while the copy arm copies between already allocated slices. The copy measurement excludes buffer acquisition, policy and TCP, so the ratio is evidence for the size of this particular copy, rather than a prediction of the saving for arbitrary UPDATEs.

Ze pays that copy for each later destination which reuses a rebuild. The loop collects pending sends and dispatches them after the comparisons have finished. Each dispatched send can then complete and return its buffer independently, without a reference count shared by the equivalent rebuilt sends.

Long queues have another ownership boundary. Items moved into overflow take their own copies, including otherwise unchanged source bodies, because they can wait longer than the receive cache's retention safety valve. [How Ze manages memory](../how-ze-manages-memory/) follows that lifetime and explains the pool limits and allocation fallbacks. Separate rebuilt output buffers do not remove the need to account for borrowed input.

## Configured announcements use their build inputs

A route created through configuration or the API has no received body to edit. Ze groups established destinations using [the same per-peer facts passed to the batch builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch.go). The resolved next hop and session role are part of those facts, as are the encoding and message-grouping settings. Within one batch, equal facts permit one build for those destinations.

This avoids keeping a second list of fields solely for the reuse key. A new per-peer decision has to become an input to the builder, and that input is also part of the comparison. Received-route reuse instead compares the immutable base and completed edits. Both approaches need the whole input to the operation being shared; neither can infer equality from a convenient subset of peer settings.

## What the fan-out benchmark covers

[BenchmarkFanoutDedup](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_bench_test.go) uses the same route fixture, with per-peer next-hop policy producing the requested number of distinct results. It compares the forwarding path with edit-set reuse enabled and disabled in one harness. There is an untimed warm-up before the measurement, and the harness exercises forwarding and pool dispatch rather than a live TCP connection or the network receive loop.

The original article reported the following Apple M4 Max measurements, with six runs per case in alternating order and the median time per destination. They are retained here as historical results, rather than new measurements of the current tree.

| Destinations | Distinct results | Reuse disabled | Reuse enabled | Change |
|---:|---:|---:|---:|---:|
| 2 | 1 | 1,228 ns | 1,090 ns | -11.2% |
| 2 | 2 | 1,233 ns | 1,286 ns | +4.3% |
| 10 | 2 | 1,234 ns | 989 ns | -19.8% |
| 100 | 2 | 1,100 ns | 729 ns | -33.7% |
| 100 | 100 | 1,102 ns | 1,115 ns | +1.2% |

The benchmark counts body rebuilds as well as time. With reuse disabled it rebuilds once per destination; with reuse enabled the intended count is once per distinct result. Allocations across this wider harness remained the same in the reported runs, because the stages surrounding the rebuild still allocate. The separate [warm rebuild checks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go) cover allocation within the encoder when output storage is available.

When every destination needs different bytes, recording and comparing answers adds 1.2 to 4.3 per cent in these results. The saving depends on repeated answers, and neither the favourable hundred-peer case nor the small-copy comparison establishes TCP throughput, route convergence or complete-router performance. The [architecture record](https://github.com/ze-software/ze/blob/main/docs/architecture/bgp/fanout-dedup.md) contains an earlier measurement set with different timings and percentages; it should not be read as the raw record for the table above.

The current encoder still walks the plan once to size it and again to write it. Some lengths are known while policy edits are recorded, so carrying them forward may remove part of that repeated calculation. For now, the size and write share the same emission walk, which keeps a change to the output from updating one calculation and forgetting the other.

*Last updated: 8 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/one-bgp-update-many-peers.md).*
