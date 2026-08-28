---
kind: directive
level: MUST
stage:
---
Where the table above accepts a string key, the string MUST reach the map
through a named constant, or a variable holding one, and MUST NOT be written as
a literal at the call site. Declare it once beside the map or the registry it
addresses, and index through that name everywhere.

A literal key is unchecked by every tool the repository runs. The compiler
compares the bytes you wrote against the bytes you meant and has no way to know
they differ; `go vet` and the linters see a valid expression. A misspelled key
on a write creates a second entry nobody reads, and on a read returns the zero
value, so the defect surfaces as absent behaviour rather than as an error: the
feature silently does nothing, and the test written to observe it passes for the
wrong reason or loses the output it was watching for.

This has already recurred here often enough to earn a gate. A log subsystem was
registered as `bgp-adj-rib-in` when the real key is `ze.log.bgp.adj.rib.in`; the
key set nothing, the level stayed at the WARN default, and the test quietly lost
the lines it existed to assert on. `internal/le/docwiring/docwiring.go` was
written to catch that one shape after it landed three times, and it can only
check hyphen-bearing subsystems declared literally in Go source. A constant
would have made all three a compile error at the first occurrence.

The rule binds hardest where a key crosses a seam and no reader ever sees the
two spellings side by side: log subsystems, env var names, YANG paths, RPC and
command names, metric names, plugin names, feature-gate tags. It is not about
avoiding strings -- the surfaces above are the ones where a string is correct --
it is about spelling each one exactly once.
