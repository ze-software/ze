---
title: One BGP UPDATE, many peers
date: 2026-08-04
author: Thomas Mangin
description: Ze's new forwarding path could rebuild the same UPDATE a hundred times. Why I chose to compare completed peer decisions, reuse the rebuild and keep the final copy.

deck: A hundred peers may need only two different UPDATE bodies. Each still has its own routing decision to make, and sharing a rebuilt buffer would tie its lifetime to the slowest send.

image: assets/blog/one-bgp-update-many-peers.svg
image-dark: assets/blog/one-bgp-update-many-peers-dark.svg
image-alt: One received BGP UPDATE passes through 100 independent peer decisions, producing two distinct encodings copied into separate peer-owned buffers.

---

I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp) to let an ordinary process speak BGP. It could announce a service or anycast prefix, inject a blackhole or FlowSpec rule, and turn received messages into text or JSON for another program. The configuration or the program using its API decided what to announce, and ExaBGP looked after the sessions.

That was a deliberate division of responsibility. An external program could decide to take a route from one peer and forward it to others, but ExaBGP did not supply an internal routing path to make that decision for it. A useful tool does not have to be a complete router.

Ze began with a migration path for those users, with a compiled, multithreaded engine which could keep their configuration and process integrations. Adding a native RIB and policy, including route-server and route-reflection behaviour, gave me rather more to design. These were responsibilities I had deliberately kept outside ExaBGP.

One of them is easy to describe. An UPDATE arrives from one peer and may leave through a hundred others, each with its own policy and encoding requirements. If those hundred decisions produce only two different bodies, rebuilding the same two answers a hundred times is a waste. I wanted to remove that repetition without making the peers share a routing decision, or making their sends depend on the lifetime of one output buffer.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude implemented the result comparison, generated AS path test cases and ran the competing benchmarks, as well as helping organise and draft the original text.*

## Doing the same thing again

For a route server, considering the UPDATE for each destination is the fan-out. A destination's policy can suppress the route, replace its next hop or edit its communities and AS path. Protocol rules and negotiated capabilities affect the result too, so belonging to the same peer group does not remove the need to decide what each peer can receive.

The straightforward implementation makes that decision and then builds a body, once for every destination. In the hundred-peer example, a hundred decisions followed by a hundred builds gives the right answer, even if there are only two distinct results. The example is illustrative, rather than a measurement from a deployment, but there is no need for a large routing table to see the repeated work.

Ze's first implementation also revisited attributes within each build and produced temporary values which then had to be copied into output. Queued and immediate announcements had separate attribute writers. There was already a reuse check, but it compared materialised wire objects, after the build it was supposed to save. The [fan-out architecture record](https://github.com/ze-software/ze/blob/main/docs/architecture/bgp/fanout-dedup.md) describes that earlier arrangement.

Go can perform all of this faster than Python, but doing unnecessary work faster was a poor reason to keep it. The comparison belonged after Ze had finished deciding what to send and before it built the bytes. Moving it any earlier would risk treating peers as equivalent while there was still policy left to apply.

## Decide first, then build

The received UPDATE is available as an immutable wire representation. Ze can inspect its path attributes and prefixes without copying the complete route into decoded objects, and the [forwarding path](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_forward.go) records the changes required for each destination by protocol rules and export policy.

Those changes may replace or remove attributes, edit the AS path, or alter which prefixes are announced and withdrawn. Policy can also suppress the route altogether. By the time encoding starts, the decision has to be complete, and a failure to encode a required change must suppress that destination's send. Falling back to the unchanged source would send the very route the policy was supposed to alter.

```text
immutable source UPDATE requiring announcement edits
    -> complete protocol and policy edits for this peer
    -> compare the source identity and the complete edits
    -> reuse a matching rebuild, or encode a new body
    -> keep the rebuilt bytes in this peer's own buffer
    -> finish destination-specific framing and queue its send
```

The reusable operation builds the content after the fixed BGP header. Splitting and some encoding-context work still happen afterwards for each destination. Two successful reusable rebuilds can therefore replace a hundred rebuilds in the example, while the hundred peers keep their own sends and the work which still depends on them.

This is usually called delayed materialisation. I find the name less interesting than the condition it imposes: the comparison must include everything the rebuild will read. Two peers agreeing so far is of no use if a later edit changes one of their answers.

## An empty list can mean two different things

Ze serialises the completed edits into a stable byte representation, called a [digest](https://github.com/ze-software/ze/blob/main/internal/component/bgp/filterapi/fingerprint.go). Values carry their lengths and operations keep the order used by the rebuild, otherwise the same bytes could describe different instructions.

Presence matters as well as content. Leaving the advertised prefixes alone and replacing them with an empty list could both look like an empty slice in a careless comparison. One preserves the announcements and the other removes them, so the digest records that distinction. This is the sort of small omission which could make a reuse test pass while changing the route being sent.

The AS path generator needs similar care. Its object is reused between peers, so its address can remain unchanged while the path it describes changes. Comparing the address would establish only that Ze was using the same working storage again. The digest includes the bytes the generator would produce.

Even a complete digest is insufficient without its input. The [reuse table](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup.go) requires the same immutable source UPDATE object, because applying equal edits to different routes can produce different results. Export policy can provide a replacement source for one destination. Separate objects containing equal source bytes currently rebuild separately, which loses a possible reuse but avoids assuming that two routes are interchangeable.

Comparing every digest against every earlier one would add its own expense, so Ze uses a 64-bit fingerprint to find likely matches, then compares each candidate's full digest byte for byte. A hash collision adds comparison work and increments a counter, but it cannot authorise reuse. An unlucky hash must never select a route.

That full comparison also leaves the hash free to be cheap. The first version used FNV-1a, with a multiply for each byte dependent on the previous result, and a [48-byte digest was recorded at around 35 ns](https://github.com/ze-software/ze/blob/main/internal/component/bgp/filterapi/fingerprint.go). Every destination paid that cost, including the ones which found nothing to reuse. The current mixing function processes eight bytes per round, while complete equality still supplies the correctness check.

The table lives for one forwarding call. The first destination with a new result causes a rebuild, and later ones with the same source and digest can copy it. There is room for at most 128 classes, up to 64 KiB of retained digests, and no individual digest larger than 2 KiB.

These descriptions include values which can come from the network, so retaining or hashing unlimited amounts for every destination would be a rather unfortunate optimisation. An edit set which exceeds the digest limit bypasses reuse. When the table cannot record another class, Ze counts the refusal and keeps the peer's independently built result; entries already recorded remain available to later peers.

## New results still need an encoder

Moving the comparison saves builds only when results match. For the first destination, or for a destination which needs different bytes, the builder still has to do its job. There was no reason to leave the temporary copies in that path.

The [body builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build.go) indexes the source attributes and plans which regions to retain, replace or remove. It walks the plan to calculate the exact output length, acquires storage, then walks it again to write. If policy leaves the communities alone, their bytes go straight from the source into the destination buffer. Even within an edited community list, retained runs can be copied directly without assembling a temporary version of the whole list first.

Exact sizing replaced a guess about spare capacity. It also lets the builder refuse an unsupported body size before writing, with failure attached to the modification which could not be made. The send is suppressed rather than rescued by sending the original route.

The [configured and queued announcement writer](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/announce_build.go) now uses the same attribute-emission machinery. With separate writers, changing the way an announcement was scheduled could also select a different implementation of where its attributes belonged. Maintaining both was an unnecessary opportunity for them to disagree.

An unchanged UPDATE can avoid this rebuilding entirely. Where encoding contexts match and no path-identifier rewrite or split is required, [the forwarding body builder already borrows the original body](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go). An earlier version of this article called that future work, which was wrong. The source stays immutable and the receive cache supplies its lifetime; the copy discussed below is the one between peers reusing a rebuilt result.

## The copy was cheap enough to keep

Sharing one rebuilt buffer among all equivalent destinations would remove that copy too. It would also mean keeping the buffer until every send using it had finished, including the slowest. Each peer already had its own output storage and could release it independently, so I was reluctant to give that up without a measurement showing a worthwhile gain.

Claude ran the comparison. In the [original published measurements](https://github.com/ze-software/ze/blob/93faff0744e0801c2d22cf17060bc5496b66863c/website/blog/posts/one-bgp-update-many-peers.md), rebuilding took 426.85 ns and copying the result took 2.07 ns, about half of one per cent of the rebuild time. That made keeping the copy an easy choice for this case.

It was a small case. [BenchmarkFanoutRebuildOnly](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_bench_test.go) uses a [fixture](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_test.go) announcing `10.0.0.0/24` and `10.0.1.0/24`, with ORIGIN, a three-AS path, NEXT_HOP, MED, LOCAL_PREF and eight communities. The body is 89 bytes excluding the BGP header, and replacing the next hop with `10.99.0.1` leaves its size unchanged. There is no MP_REACH or attribute 40 in the fixture for the other recorded edits to change.

The historical rebuild arm has no peer output pool and includes its owned-result allocation fallback. The copy arm uses already allocated slices, so its timing excludes buffer acquisition as well as policy and TCP. The ratio answers how much complexity this particular copy justified; it cannot predict savings for arbitrary UPDATEs.

Ze therefore copies a reused rebuild into each later destination's own output buffer. The loop collects pending sends and dispatches them after all comparisons have finished, so none can return a buffer while the table still needs it as a copy source. Once dispatched, the rebuilt sends finish independently. I would rather pay for that small copy than make all those sends agree on when shared output storage can be released.

Overflow still needs a separate rule. Items moved into overflow take owned copies, including unchanged bodies which could otherwise borrow their source, because the queue can outlive the receive cache's retention safety valve. [How Ze manages memory](../how-ze-manages-memory/) follows that lifetime and the pool allocation fallbacks. Giving rebuilt output an independent owner does not let a queued borrowed input live forever.

## Configured routes have different inputs

Configured and API announcements are closer to ExaBGP's usual workload. There is no received body to edit: Ze already has the route to announce and needs to establish which destinations require the same build. A source UPDATE and an edit digest would be the wrong inputs to compare here.

Instead, the configured path groups established destinations using [the same per-peer facts passed to the batch builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_api_batch.go). Those include the resolved next hop and session role, along with encoding and message-grouping settings. Within a batch, equal facts permit a shared build when grouping is enabled.

I kept the builder's inputs and the grouping key as the same description. A second list maintained just for comparison would mean remembering to update both whenever a new per-peer input was added, and forgetting one field could group peers which need different bytes. The current structure brings that new input into the comparison too.

Received routes thus compare their immutable source and completed edits, while configured routes compare their build inputs. A peer group or a matching OPEN exchange supplies only part of the information. Neither says that all the subsequent decisions will produce equal output.

## What the forwarding benchmark measured

The [fan-out benchmark](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_dedup_bench_test.go) uses the same route fixture, with per-peer next-hop policy generating the requested number of distinct results. It runs the forwarding path with edit-set reuse enabled and disabled in one harness, after an untimed warm-up. Forwarding and pool dispatch are included, while a live TCP connection and the network receive loop are not.

That is the part of the program this change affects, and the harness also counts rebuilds so that a faster time can be checked against the operation being removed. With reuse disabled it rebuilds once per destination; with reuse enabled the intended count is once per distinct result.

The original article reported these Apple M4 Max results from six runs per case in alternating order. Times are medians per destination. They remain historical measurements, rather than a fresh benchmark of the current tree; the [architecture record](https://github.com/ze-software/ze/blob/main/docs/architecture/bgp/fanout-dedup.md) contains an earlier set with different timings and percentages, rather than the raw record for this table.

| Destinations | Distinct results | Reuse disabled | Reuse enabled | Change |
|---:|---:|---:|---:|---:|
| 2 | 1 | 1,228 ns | 1,090 ns | -11.2% |
| 2 | 2 | 1,233 ns | 1,286 ns | +4.3% |
| 10 | 2 | 1,234 ns | 989 ns | -19.8% |
| 100 | 2 | 1,100 ns | 729 ns | -33.7% |
| 100 | 100 | 1,102 ns | 1,115 ns | +1.2% |

When every destination needs different bytes, recording and comparing the results adds work and saves no builds. That cost was 1.2 to 4.3 per cent in these runs. When a hundred destinations needed two results, the saved builds reduced time per destination by 33.7 per cent. These figures do not establish TCP throughput, route convergence or complete-router performance.

Allocations across the wider harness stayed the same, because stages surrounding the rebuild still allocate. The separate [warm rebuild checks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go) cover the encoder with output storage available. Reusing builds and eliminating allocations from a complete forwarding operation are different jobs.

## Where I stopped

The encoder still walks its plan twice, once for size and once to write. Some lengths are known when edits are recorded and could be carried forward, but for now sizing and writing use the same emission walk. Keeping those two operations in agreement is worth something too, and another arrangement needs to save enough to justify maintaining it.

Changing the original source buffer would be a more demanding step. Every later reader and policy decision would have to be finished with its bytes, there would have to be room for the modification, and ownership would have to pass safely to the sends. I kept the source immutable. Eligible unchanged sends already borrow it, and changed sends can reuse the rebuilt result without any right to alter the input.

Claude did the legwork on the comparison, the generated AS path cases and the competing benchmarks. That makes this kind of implementation practical for a solo developer, while I decide which reuse is safe and which failure must stop a route from leaving. In this case the useful saving was avoiding repeated builds, and the copy was cheap enough to keep. I left each rebuilt send with its own buffer.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/one-bgp-update-many-peers.md).*
