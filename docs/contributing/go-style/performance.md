# Ze Go Style: performance

The detail behind "Performance" in [the Go style core](../ze-go-style.md),
which states the rule.

- **Sketch the four resources:** network, disk, memory, CPU, each with its
  bandwidth and its latency. Weight each by how often you touch it: a thousand
  cache misses cost more than one disk write.
- **Separate the control plane from the data plane.** A session negotiation runs
  once for each peer, and an UPDATE runs millions of times. The control plane
  can afford a check that the data plane cannot.
- **Batch,** for a system call, a disk write, a lock, and a wake-up alike, and
  give the CPU a large piece of work with a predictable shape.
- **Do not depend on the compiler.** Extract a hot loop into a function that
  takes primitive arguments and no receiver, so no struct field has to be
  proved cacheable and a redundant computation is visible.
- **Prove a compiler optimization from the compiler output.** Inlining is not
  that proof: an inlined call still executes its body. `-gcflags=-m` shows the
  inlining and the escapes, and `-gcflags=-S` shows the instructions. The passes
  that remove a redundant copy run in an order that changes between Go
  releases, so run the hot-path benchmarks again after each toolchain bump.
- **Change the form of a value once.** A conversion goes from the source form
  to the destination form. It does not go through a third form: a typed value
  to bytes and back to a typed value, or wire bytes to a struct and back to
  wire bytes. When the type has no direct operation, add the operation to the
  type that owns the representation. Bernat measured the round trip
  `netip.AddrFrom16(a.As16())` at eight times the cost of a direct method
  (<https://vincent.bernat.ch/en/blog/2026-go-netip-addrto6>).

## Zero allocation on a wire path

| Rule | Detail |
|------|--------|
| The caller owns the buffer | A callee writes into `buf[off:]` and returns the byte count. It allocates nothing |
| A pool replaces `make` | A wire path takes its buffer from a bounded pool. `make` stays for a fixed-size header and a one-shot allocation at startup |
| The pool shape follows the goroutine shape | One sequential goroutine takes a ring. Concurrent goroutines take a `sync.Pool` seeded for the peak |
| One maximum size for every buffer in a pool | A variable size defeats the pool |
| A string on a hot path is built once | Concatenation with `+`, `strings.Join`, and a `map[string]V` key each cost an allocation. Use one `textbuf.Buffer` and a typed key parsed at the boundary |
| A value that escapes to the heap | The caller passes an out pointer |

<!-- source: internal/core/textbuf/textbuf.go -- Buffer -->
