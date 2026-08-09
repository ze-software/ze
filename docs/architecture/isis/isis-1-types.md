# IS-IS Domain Types

`internal/plugins/isis/types` is the leaf package every higher IS-IS layer keys
on. It models the eight domain types of ISO/IEC 10589 sections 1.4 and 6.2 with
RFC 5305 and RFC 5308 metrics: `SystemID`, `SourceID`, `LSPID`, `NET`, `AreaID`,
`Metric`, `PrefixMetric`, `SequenceNumber`, and `RemainingLifetime` /
`HoldingTime`.

It has no network I/O, no timers, no goroutines, and no import from the IS-IS
runtime. Its dependency closure is `errors`, `bytes` and `strings`. The layering
is `types <- packet <- runtime`; see
[`../wire/isis.md`](../wire/isis.md) for the codec above it.

## Decision: fixed arrays for the identifiers

`SystemID` is `[6]byte`, `SourceID` is `[7]byte`, `LSPID` is `[8]byte`. They are
arrays rather than slices or pointers so they compare with `==` and serve
directly as Go map keys. The adjacency table and the LSDB index rely on that: no
wrapper struct, no pointer-identity surprise.

The variable-length `AreaID` (1 to 13 octets) and `NET` (8 to 20 octets) are
slice-backed structs with explicit `Equal` and `Compare`.

<!-- source: internal/plugins/isis/types/systemid.go -- SystemID -->
<!-- source: internal/plugins/isis/types/lspid.go -- LSPID, Compare, Less -->

## Decision: two metric widths, never conflated

`Metric` is 24-bit and serializes in 3 octets: the TLV 22 IS-reachability metric.
`PrefixMetric` is the full 32-bit field and serializes in 4 octets: the TLV 135
and TLV 236 prefix metric. Capping a prefix metric at 24 bits would reject or
mangle valid peer routes, so the widths are separate types rather than one type
with a range check.

The two constructors have different error surfaces on purpose:
`MetricFromBytes` cannot range-error because 3 octets cannot exceed 24 bits,
while `NewMetric` can because a caller can pass a larger `uint32`.

The narrow 6-bit metric is not modeled. Ze originates wide metrics only.

<!-- source: internal/plugins/isis/types/metric.go -- Metric, PrefixMetric, NewMetric, MetricFromBytes -->

## Decision: sequence zero and purge are orthogonal

`SequenceNumber(0)` reports `IsReserved()`. Origination starts at
`FirstSequenceNumber` (1). `Next` and `NextChecked` never produce 0: on wrap past
the 32-bit maximum they return 1, and `NextChecked` reports that it wrapped so
the LSDB can run its purge-then-suspend-then-re-originate sequence.

A purge is a **separate** signal: `RemainingLifetime` 0, reported by `IsPurge()`.
It is never sequence 0. Keeping these orthogonal is the load-bearing correctness
call in this package; conflating them causes origination loops downstream.

<!-- source: internal/plugins/isis/types/sequence.go -- SequenceNumber, IsReserved, Next, NextChecked -->
<!-- source: internal/plugins/isis/types/lifetime.go -- RemainingLifetime, IsPurge, HoldingTime -->

## Decision: buffer-first, zero-allocation formatting

Every type has `WriteTo(buf, off) int` that writes big-endian octets into a
caller buffer. Display uses shared append helpers that write into a caller
scratch array. `AppendTo` is asserted to allocate zero times; `String()` costs
the one unavoidable result copy. No path calls `fmt.Sprintf`.

Do not tighten the `String()` allocation bound to zero: an owned string is one
allocation by definition. Hot paths call `AppendTo`.

## Decision: explicit, documented ordering

`LSPID.Compare` and `Less` are big-endian over all 8 octets, which is the exact
order CSNP and PSNP use to bound an LSP-entry range. `AreaID.Compare` is
byte-lexicographic through `bytes.Compare`, so a shorter prefix sorts first. Both
orderings are load-bearing for area match and CSNP range bounds.

## Trap: parse rejects the wrong grouping

`parseDottedHex` rejects a string with the right number of hex digits and the
wrong grouping, for example `000100020003` with no dots. The canonical IS-IS
identifier form is dot-grouped. Do not relax this into accepting undotted hex.

<!-- source: internal/plugins/isis/types/format.go -- parseDottedHex, appendDottedHex, parseHexOctets -->
<!-- source: internal/plugins/isis/types/net.go -- appendNETHex -->

## Trap: the NET split is positional and its text form differs

`NET` accessors slice the trailing 7 octets as SystemID plus SEL and everything
before as the AreaID. There is no delimiter to key on.

The NET text form groups the first octet alone and then in pairs
(`49.0001.0000.0000.0001.00`). That is a **different** grouping from the plain
dotted hex used by `SystemID` and `LSPID`. Two append helpers exist for this
reason. Do not unify them.
