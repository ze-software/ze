# Rationale: No Sprintf Allocations

Ze processes millions of BGP UPDATEs per second. `fmt.Sprintf` allocates
2-3 times per call: format string parsing into a `pp` struct, intermediate
buffer growth, and the result string. On a per-UPDATE path called millions
of times, that is millions of unnecessary GC objects per second.

The alternative (`textbuf.Buffer` with a 128-byte stack-inline array) does
one allocation total for the final `String()`. For values that stay as
`[]byte`, it does zero. The asymmetry is extreme: replacing Sprintf costs
30 seconds of developer time, but leaving it costs measurable latency and
GC pressure in production forever.

The rule exists because every AI session's trained instinct is to reach for
`fmt.Sprintf`. It is the first formatting tool Go developers learn, it
appears in every tutorial, and it works correctly. The problem is purely
performance: correct but slow, on a path where slow means dropping routes.

`fmt.Errorf` with `%w` is the one exception because error wrapping is its
designed purpose and errors are cold-path by definition (they represent
failures, not the happy path). Sentinel errors (`errors.New` at package
level) are preferred when the format string is constant.
