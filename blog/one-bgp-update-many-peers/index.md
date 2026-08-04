# One BGP UPDATE, many peers

*2026-08-04*

[ExaBGP](https://github.com/Exa-Networks/exabgp) was never meant to be a router. I wrote it so an ordinary process could speak BGP: announce a service or anycast prefix, inject a blackhole or FlowSpec rule, and turn received messages into text or JSON that another program could use. Most ExaBGP installations use it as a route announcer, and that narrow focus is still one of its strengths.

There are optional Adj-RIB-In and Adj-RIB-Out caches in the code, mostly to support a peer session and replay announcements. ExaBGP has no Loc-RIB, performs no best-path selection, does not itself forward a route learned from one peer to another, and does not manipulate the FIB. The routing decision belongs to the configuration, or to the process using its API.

That division of responsibility has solved real operational problems for many years. Its limits also became clear when people wanted to announce large sets of routes, or decode and report a full table. Python's interpreter and object model set a ceiling which careful code could move, though never remove.

Ze began by giving ExaBGP users a path to a compiled, multithreaded engine without forcing them to replace their configuration and their process integrations. Its scope then grew. Ze gained a native RIB, policy, route-server behaviour and route reflection. Those features cross a boundary which ExaBGP deliberately leaves to external programs.

That distinction matters for this article. The one-to-many forwarding path described here was not ported from ExaBGP, because ExaBGP has no equivalent internal path. It is new work required by Ze's broader role: one route can enter through one peer, pass through Ze's policy, and leave through many others.

The problem looks simple, though it touches protocol rules, export policy, memory ownership and performance. It is also a good example of how we build Ze. Experience defines the architecture, its invariants and its limits. Claude does most of the implementation, test generation, benchmarking and revision needed to turn that design vision into working code. Without that legwork, realising the design as a solo developer would have been much harder.

*This article was co-authored with Claude. The architecture, design decisions, measurements and conclusions come from my work on ExaBGP and Ze. Claude helped organise the material and draft the text.*

## The new problem: one route, many destinations

Imagine Ze acting as a route server. It receives one UPDATE and evaluates the route for 100 client peers. Every client still needs an independent policy decision. Some policies suppress the route; others add or remove communities or alter the AS path. Their OPEN exchanges may also have negotiated different capabilities, which can change how the UPDATE is encoded.

The loop which takes that route past all 100 peers is the fan-out. The simplest implementation generates a complete UPDATE for every destination. If policy produces only two distinct messages, it still performs the full generation 100 times.

Ze's first implementation kept revisiting the same message. It scanned the UPDATE to locate the AS path, the communities and the next hop. It built temporary copies of the values changed by policy, then copied them again into the output. Converting an AS path required another pass over the message. A route queued before a session was ready and a route announced afterwards used separate code to produce their BGP bytes. Ze looked for identical results only after it had already produced both of them. Each of those choices was reasonable on its own, and together they repeated a great deal of work.

Go made each build cheaper, but Ze still rebuilt the same result for every destination. If 100 independent peer decisions produce only two final UPDATEs, Ze should encode two bodies and not 100. Each peer's policy and negotiated capabilities must still be evaluated independently, and each peer keeps its own send operation.

## Wait until policy is complete

The UPDATE received from a peer is already a complete BGP message. Ze keeps its bytes unchanged and reads information directly from them when needed. It does not duplicate the whole route into another in-memory structure first.

BGP calls the information carried with a route its path attributes. The AS path, the communities and the next hop are familiar examples. The UPDATE also lists the prefixes being announced or withdrawn.

For each destination peer, Ze applies the protocol rules and the export policy, then records every required change in a list. That list might say: use a different next hop, add a community, prepend the local AS, remove an attribute, replace the advertised prefixes, or send a withdrawal instead.

Ze produces no BGP message while these decisions are being made. The complete path is:

```text
received UPDATE
    -> make every protocol and policy decision for one peer
    -> complete the list of changes
    -> check whether an earlier peer needs the same result
    -> encode the UPDATE content if this result is new
    -> copy it into this peer's output buffer
```

The encoded content is the part of the UPDATE after the fixed BGP header. Each peer's send queue has its own output buffer, a reusable block of memory which it keeps until that peer has sent the message. It can then reuse the buffer for another send.

Turning the decisions into encoded content is often called *materialisation*. Delaying it is what makes everything after it possible, and the rule holds well outside Ze: two destinations can reuse encoding work only once the system knows everything which may change what either of them receives.

## Reuse a completed answer when it matches

How can Ze tell that two peers need exactly the same bytes? It compares both the received UPDATE and the complete list of changes.

To make that comparison efficient, Ze writes the change list in one stable form called a digest. It records whether the route becomes a withdrawal, the exact prefixes being announced or withdrawn, and every change to values such as the next hop, the communities and the AS path. The order and the length of each value are included, so two different instructions cannot accidentally look alike.

The received UPDATE is part of the comparison too. Applying the same changes to two different routes does not necessarily produce the same result.

Ze first calculates a short 64-bit fingerprint of the digest and uses that number to narrow the search to likely matches. Two different digests can occasionally produce the same fingerprint, which is called a collision. Ze therefore compares the complete digests byte for byte before reusing any result.

The two steps have different authority. The fingerprint selects the candidates and the comparison authorises the reuse, with no shortcut when the fingerprint happens to match, because reusing on a fingerprint alone would eventually hand one peer another peer's bytes. A collision costs one unnecessary comparison and nothing else. Ze counts those collisions, so a hash which stops separating its inputs shows up in the statistics instead of quietly wasting comparisons.

Because the comparison carries the correctness, the fingerprint itself can be cheap. It mixes eight bytes of the digest per round. The first version used FNV-1a, which performs one multiply per byte with each multiply waiting on the result of the previous one; on a 48-byte digest that cost around 35 nanoseconds for every destination, hit or miss, which is roughly a tenth of the rebuild the whole mechanism exists to avoid.

The first peer needing a new result causes Ze to encode the UPDATE content once. A later peer starting from the same received UPDATE and requiring the same changes copies that content into its own output buffer. In our example, all 100 peers still receive their own policy decision and their own send operation, while two distinct results require only two encoding operations.

For each received UPDATE, Ze temporarily remembers at most 128 generated results and up to 64 KiB of their digests; no individual digest may exceed 2 KiB. The digest of a route carrying a 65,535-octet community list would cost more to hash than the rebuild it is meant to save, and the input comes from the network, so the ceiling is not optional. If any limit is reached, Ze encodes the affected peer's result independently and increments a counter showing that the optimisation was skipped.

## Encode the result directly

Once Ze knows that a result is new, it encodes the content in three steps.

1. **Plan.** Read the original UPDATE once and decide which parts can be copied unchanged, which must disappear, and which need replacement.
2. **Size.** Calculate the exact length of the finished content, including every replacement.
3. **Write.** Obtain an output buffer of that size and write the unchanged and replacement bytes directly into it.

Suppose the communities stay unchanged while another part of the UPDATE needs rebuilding. Ze copies their original bytes straight into the output buffer. If an AS path must be converted for a peer without four-octet AS number support, Ze writes the converted path directly into the same buffer. Neither case creates a temporary copy of the whole community list or of the AS path.

If the complete UPDATE needs no change and the destination accepts the same encoding, Ze could send from the original receive buffer and avoid rebuilding it. We have left that optimisation for later. The case is uncommon on eBGP sessions, and retaining the receive buffer through each peer's send would complicate an ownership model which is currently simple.

The previous generator guessed the output size and requested extra memory which might never be used. Ze now calculates the exact encoded length first. Received routes, configured routes and routes queued while a session is starting all use the same encoding rules, so identical route and peer inputs produce identical bytes regardless of how or when the route entered Ze.

## Why copy instead of share

The obvious first design is to let every equivalent peer share the same generated bytes. It appears to remove even the final copy.

Each peer sends at its own pace. If peers shared one output buffer, Ze would have to count how many sends were still using it and wait for the slowest peer before reusing it. One congested peer could hold the shared buffer for every other peer in the group. Separate buffers avoid that coordination.

We measured the two operations before accepting that complexity. Rebuilding the test UPDATE took a median 426.85 nanoseconds. Copying its encoded content into another peer's output buffer took 2.07 nanoseconds. The copy represented about half of one percent of the rebuild cost.

Half of one percent is still half of one percent, paid once per destination. At a hundred destinations that is a hundred copies a shared buffer would not have made. Ze pays it today. The first encoded result becomes the source for copies, and each peer receives a copy in its own output buffer, so one queued send owns one buffer and releases it when done. Each peer can also split large content into legal-sized BGP UPDATE messages according to its negotiated limit, without coordinating with another peer.

Separate buffers keep almost all of the available improvement and leave the ownership rule small enough to check by reading it.

Ze is a work in progress and not every design decision is frozen. The open question here is whether the comparison needs a hash at all. Peers announce their characteristics during the OPEN exchange, and those characteristics already decide much of what each destination can receive. Indexing on them directly would group destinations before any digest exists, which is what the configured-route path further down already does with its build key.

## The promise to reuse memory

Memory management is the subject of the [first article in this series](../how-ze-manages-memory/). One promise matters here. After initial traffic has created reusable output buffers, a normal peer-specific change should reuse them instead of requesting another block of Go-managed memory.

Ze also clears and reuses the list of changes between peers. An unusually large UPDATE, or the absence of a suitable buffer, may still require fresh memory; those are safe fallbacks outside the promise. We verify normal UPDATE encoding separately from the wider forwarding path, because the stages before and after encoding may still request memory.

## Configured announcements take a parallel path

This path is closer to ExaBGP's usual workload. A route created through the API or the configuration has no received UPDATE to modify. Ze already knows the route it wants to announce, and different peers may still need different bytes.

During the OPEN exchange, peers agree on capabilities which affect the message format. Four-octet AS number support changes how an AS path may be represented. ADD-PATH adds a path identifier when multiple paths for the same prefix are advertised. Extended Next Hop can change how the next-hop address is encoded. The result may also depend on the resolved next hop, whether the session is internal or external, route-server behaviour, the local AS number and the largest message the peer accepts.

Those inputs form a build key. Peers with the same key reuse one encoding operation: Ze builds the bytes once and copies them into each peer's own output buffer. They never share a live buffer. A different key causes a separate encoding. For received routes, equality comes from the original UPDATE and the complete change list; for configured and API routes, it comes from every input needed to build a new UPDATE. Both paths reuse work only when Ze can prove that the resulting bytes will be equal.

## What we measured

The fan-out benchmark isolates one question: what happens when one received UPDATE must go to several peers? The previous code encoded new content after every peer's policy decision. The current code still makes every policy decision, then reuses encoding when two complete results are equal. Receiving the route, applying filters, obtaining an output buffer and placing the result on the send queue are the same in both versions.

These measurements were taken on an Apple M4 Max. Each version ran six times for every case, in alternating order, and the table reports the median time per destination.

| Destinations | Distinct results | Previous time | Current time | Change |
|---:|---:|---:|---:|---:|
| 2 | 1 | 1,228 ns | 1,090 ns | -11.2% |
| 2 | 2 | 1,233 ns | 1,286 ns | +4.3% |
| 10 | 2 | 1,234 ns | 989 ns | -19.8% |
| 100 | 2 | 1,100 ns | 729 ns | -33.7% |
| 100 | 100 | 1,102 ns | 1,115 ns | +1.2% |

The benchmark also counts how often UPDATE content is encoded, and it confirms the intended behaviour: the previous version encoded once per destination, while the current version encodes once per distinct result. The broader benchmark still requested the same number of new Go-managed memory blocks, because stages outside UPDATE encoding also request memory. A separate, focused check verifies that normal UPDATE encoding itself reuses existing memory.

When every peer needs different bytes, recording and comparing results adds between 1.2% and 4.3% in these runs. When many peers need one of a few results, the comparison avoids repeated generation. These are measurements of this specific path. They say nothing about TCP throughput, route convergence or a complete router workload.

## A practical stopping point

The design still has room to improve. Exact sizing currently requires a pass to determine the output length followed by another pass to write the bytes. Some lengths are already known while the list of changes is being built, so we may be able to carry that information forward and remove part of the repeated calculation. The current separation makes size and write easy to compare and test, which is why we kept it for now.

There is a more ambitious opportunity when every destination belongs to one peer group, every relevant setting comes from that group, and all the peers have negotiated the same encoding. Once Ze has established that no destination needs a different result, it could apply the common change directly to the received buffer and use that buffer as the first result. If nothing needs changing at all, the original content could go out as it arrived.

That optimisation would require a stronger ownership proof. No policy decision or later reader could still need the original bytes, the buffer would need enough room for the change, and its lifetime would have to move safely from the receive side to the send side. Keeping the input unchanged avoids those conditions and is easier to reason about.

We stopped at a practical boundary for now: immutable input, one encoding per proven result, and separate output buffers for peer sends. It removes the large repeated cost while keeping failures and memory lifetimes straightforward. Future measurements may justify moving that boundary again.

None of this is specific to BGP. Any system which sends one thing to many destinations can finish its per-destination decisions before it serialises anything, then compare a stable description of the whole decision, together with the identity of the source, to decide whether an earlier answer will do. A short hash finds the candidates and a full comparison approves them. Whatever state that comparison retains has to be bounded, because the input comes from the network. And before anybody reaches for shared memory to avoid a copy, they should measure the copy. Ours cost two nanoseconds.

## Where AI helped

Claude wrote the result comparison, generated many AS path test cases and ran the competing benchmarks. The difficult choices still need context. A matching short fingerprint is always followed by a full comparison, because the alternative is one peer receiving another peer's bytes. Peers keep separate output buffers because the copy costs 2 nanoseconds against a 427 nanosecond rebuild, a cost Ze pays until a better ownership design earns it back. Details like those come from operating programmable BGP systems.

We record that context before implementation: which work may be reused, who owns each piece of memory, which errors prevent a route from being sent, where fresh memory is acceptable, and which measurement can prove the result. Claude helps turn those decisions into code, test the combinations and failure cases, and review the result. When the evidence disagrees with our first idea, we change the design.

Ze keeps ExaBGP's programmable configuration and process model so its users have a practical migration path. The forwarding design in this article belongs to Ze's wider job as a routing engine. Every peer receives an independent policy decision and its own send operation, and within the reuse limits each distinct final UPDATE is generated once and copied into each matching peer's output buffer.

ExaBGP does one job clearly. Ze has taken on a broader job, and the interesting part is working out which of those lessons still apply and where the design has to begin again.
