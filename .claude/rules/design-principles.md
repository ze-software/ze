# Design Principles

Rationale: `.claude/rationale/design-principles.md`

| Principle | Rule |
|-----------|------|
| YAGNI | Don't build what's not immediately needed |
| Simplicity | Boring code that obviously works > clever code |
| No identity wrappers | Wrapper must transform (type conversion, error wrapping, defaults). Delete if just delegates. A struct holding raw data + accessor methods is an identity wrapper -- pass the data, use existing type methods directly |
| Single responsibility | One thing per function/struct/package. "And" in name = split |
| Explicit > implicit | No hidden magic, convention-based behavior, silent defaults |
| Minimize coupling | Components know minimum about each other. High->low dependency |
| Interface segregation | Clients depend only on methods they use |
| No premature abstraction | Three concrete implementations before abstracting |
| Design for change | Isolate volatility behind stable interfaces |
| Fail-mode awareness | Every external call can fail. Every input can be malformed |
| Do it right | Ze does the hard thing properly -- zero-copy, pool dedup, buffer-first. Never trade correctness for speed of implementation |
| Durability over velocity | Optimize for "never revisit this code" not "get to commit fast". Missing edge cases, shallow tests, unwired features all create rework. Rework wastes more of the user's time than thoroughness ever could |
| Encapsulation onion | Networking protocols are nested encapsulations. Allocate one buffer at the outermost layer and slice inward with specialized data-manipulation types (WireUpdate, PackContext). Each layer peels the onion by narrowing the window -- never by copying into a new buffer. Currently BGP-only; the pattern holds for any future protocol layer |
| Buffer-first encoding | The write side of the onion: all wire encoding writes into pooled, bounded buffers via `WriteTo(buf, off) int`. No `append`, no `make` in helpers, no `buildFoo() []byte`. Pool buffer size = RFC max = bounded encoding space. Mechanical details: `rules/buffer-first.md` |
| No `make` where pools exist | Any runtime `make([]byte, N)` whose N is variable (header-declared length, config-driven size, user input) on a wire-facing path MUST come from a pre-allocated, bounded pool. Rationale: `make` can fail with `runtime: out of memory` under heap pressure, and the caller usually has no recovery path; a pre-allocated pool has a deterministic size at start-up, blocks callers when exhausted (backpressure), and eliminates the failure mode entirely. Scope: all subsystems that read or write framed packets off a socket (BGP, TACACS+, L2TP, PPP, any future protocol). `make` remains OK for fixed-size headers (stack-allocated by escape analysis) and for one-shot startup allocations. |
| Pool strategy by goroutine shape | Two pool strategies, each with a clear domain. (a) **Single-backing ring** (e.g., `internal/component/bgp/reactor/forward_pool.go`): `make([]byte, n*size)` once at construction, hand out N sub-slices via `backing[i*size:(i+1)*size:(i+1)*size]`. Use when the hot path is one reactor goroutine consuming and releasing in sequence -- one contiguous mapping, three-index slice caps each slot so `append` cannot spill into a neighbor, and there is no need for per-CPU cache. (b) **`sync.Pool` seeded for peak** (e.g., `internal/component/tacacs/pool.go`): seed with enough buffers to cover peak concurrent wire activity, let sync.Pool's per-P cache remove Get/Put contention. Use when the hot path has multiple concurrent goroutines (client shared by auth/authz/accounting). Under bounded traffic with correct seeding, the pool's `New` func is the last-resort fallback, not a regular allocation path. Every buffer in a given pool is the SAME MAX size so Get callers never resize. |
| Lazy over eager | The read side of the onion: pass raw byte slices, not parsed structs. Use offset-based iterators (Next() yields one element), not collected slices. Consumer walks data and acts directly -- no intermediate maps or structs built to iterate once. Never wrap raw data in a new struct with accessor methods -- use existing wire type methods or standalone functions. Optimizing N->1 is wrong when the answer is N->0-until-needed |
| Zero-copy, copy-on-modify | A buffer is allocated from the Incoming Peer Pool when a packet is received from the wire. It is never copied as it flows through the system -- filters inspect it, the forwarding path shares it across N destination peers, workers write it to TCP. The buffer is only copied when a destination peer needs modification (AS-PATH prepend, attribute rewrite). At that point, the modified copy is written into a buffer from the destination's Outgoing Peer Pool. After all destinations have sent (or copied), the source buffer is released to its Incoming Peer Pool. This is the complete memory lifecycle: allocate at receive, share read-only, copy only on modify, release after send. Adding a copy where none is needed is a bug, not a design choice. Each peer has two Peer Pools of the same type: the Incoming Peer Pool (inbound) and the Outgoing Peer Pool (outbound modification). The Global Shared Pool handles overflow when a Peer Pool is exhausted. |

## Scalability Checklist

```
[ ] No premature abstraction (3+ use cases?)
[ ] No speculative features (needed NOW?)
[ ] Single responsibility per component
[ ] Explicit behavior (no hidden magic?)
[ ] Minimal coupling
[ ] Consistent naming
[ ] Testable in isolation
[ ] Next-developer test: understood in 30 seconds?
```
