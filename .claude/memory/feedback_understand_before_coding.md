---
name: Understand the full memory lifecycle before coding
description: Trace buffer allocation through the entire system before choosing an abstraction. Wrong lifecycle = wrong design = multiple rewrites.
type: feedback
---

Before implementing any buffer/pool/allocation, trace the complete memory lifecycle through the system first.

**Why:** Went through five iterations on the Outgoing Peer Pool because I didn't understand the lifecycle upfront:
1. Channel-based pool (slow, wrong primitive)
2. Atomic counter (right for gating, wrong for holding data)
3. Pre-allocated buffers acquired at TryDispatch (wrong acquisition point -- every forward, not just modifications)
4. Realized the buffers weren't used for data at all (just expensive counters)
5. Finally understood: buffers are for copy-on-modify in egress filters, acquired in buildModifiedPayload

**How to apply:**
- Trace the full lifecycle: where is memory allocated? Who holds it? When is it copied? When released?
- Ze's lifecycle: allocate at receive (Incoming Peer Pool), share read-only through forwarding, copy only when egress filters modify (Outgoing Peer Pool), release after TCP write
- Use the correct terminology: Incoming Peer Pool, Outgoing Peer Pool, Global Shared Pool
- Zero-copy is the default. Adding a copy is a bug unless modification requires it.
- The acquisition point defines the design: "every dispatch" vs "only on modification" are fundamentally different
- A pool that provides buffers is not a counter. A counter that gates concurrency is not a pool. Know which one you need before writing code.
- Look at the filter code and buildModifiedPayload to understand WHERE modification happens before deciding WHERE buffers come from.
