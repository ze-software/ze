# 927 -- isis-1-types

## Context
Spec `isis-1-types` creates the bottom layer of the native IS-IS component:
`internal/component/isis/types/`, the pure value-type leaf package that every
higher IS-IS layer keys on (types <- packet codec <- server runtime). It models
the eight IS-IS domain types from ISO/IEC 10589 section 1.4 / 6.2 and RFC 5305 /
RFC 5308: `SystemID`, `SourceID`, `LSPID`, `NET`, `AreaID`, the two wide metric
widths `Metric` (24-bit) and `PrefixMetric` (32-bit), `SequenceNumber`, and
`RemainingLifetime` / `HoldingTime`. No network I/O, no timers, no goroutines, no
imports from the IS-IS runtime. It is the IS-IS equivalent of a self-contained
address/identifier library, consumed first by the wire codec (isis-2) and
transitively by every later IS-IS child. This was a from-scratch build; Ze had no
IS-IS types before (BGP-LS carries link-state inside BGP NLRI but does not model
these addressing types, and is deliberately not coupled to).

The implementation is DONE and verified by unit/build evidence: 38 unit/boundary
tests pass under `-race` (`go test -race -count=1 ./internal/component/isis/types/`
-> `ok ... 1.324s`), golangci-lint reports `0 issues` for the package, and the
package cross-compiles clean on darwin and linux (`go vet` both GOOS). The
remaining work is interop validation, which is pending Linux/QEMU execution (see
the note at the end). For this leaf type package interop is N/A as a direct AC:
the types carry no wire behaviour on their own; on-wire validation happens where
the codec and runtime emit/consume real frames (downstream specs).

## Decisions
- **Fixed-array value types for the identifiers.** `SystemID` ([6]byte),
  `SourceID` ([7]byte), `LSPID` ([8]byte) are fixed arrays, NOT slices/pointers,
  so they are comparable with `==` and usable directly as Go map keys (adjacency
  table, LSDB index). `TestSystemIDMapKey` / `TestLSPIDMapKey` pin this. The
  variable-length `AreaID` and `NET` are slice-backed structs (honest 1..13 /
  8..20 byte representation) with explicit `Equal`/`Compare`.
- **Two distinct metric widths, never conflated.** `Metric` is 24-bit
  (TLV 22 IS-reachability, range 0..16777215, 3-octet serialize, `NewMetric`
  rejects > max). `PrefixMetric` is 32-bit (TLV 135 / TLV 236 IP/IPv6 prefix,
  full uint32, 4-octet serialize, no error return). Capping a prefix metric at
  24-bit would reject/mangle valid peer routes, so the widths are separate types.
  `TestMetricWidthsDistinct` pins 3 vs 4 octets. The narrow 6-bit metric is NOT
  modelled (wide-only per umbrella); revisit only if isis-2 interop decode demands
  a dedicated type.
- **Sequence-zero is reserved and represented distinctly.** `SequenceNumber(0)`
  reports `IsReserved()` true; origination starts at `FirstSequenceNumber` (1).
  `Next`/`NextChecked` never produce 0 -- on wrap past the 32-bit max they return
  1 and `NextChecked` flags `wrapped` so the runtime (isis-6) can do the
  purge-then-re-originate dance. Purge is a SEPARATE signal: `RemainingLifetime`
  0 (`IsPurge()`), NOT sequence 0. Keeping these orthogonal is the single most
  load-bearing correctness call in the package.
- **Buffer-first, zero-alloc formatting.** Every type has `WriteTo(buf, off) int`
  that writes big-endian octets into a caller buffer (no per-call slice alloc).
  Display uses shared `appendDottedHex` / `appendNETHex` helpers that append into
  a caller scratch array; `AppendTo` is asserted 0-alloc and `String()` costs only
  the one unavoidable result copy (`testing.AllocsPerRun`, `format_test.go`). No
  `fmt.Sprintf` on any path.
- **Explicit, documented ordering.** `LSPID.Compare`/`Less` is big-endian over all
  8 octets (the exact order CSNP/PSNP use to bound an LSP-entry range).
  `AreaID.Compare` is byte-lexicographic via `bytes.Compare` (a shorter prefix
  sorts first), documented in the code as load-bearing for area match and CSNP
  range bounds (spec risk R-1).
- **Stdlib-only dependency closure.** `go list -deps` shows only `errors`,
  `bytes`, `strings` (+ runtime internals) and the package itself -- no IS-IS
  runtime, no BGP-LS. The planned optional `internal/core/textbuf` helper proved
  unnecessary because the dotted-hex format is self-contained.

## Consequences
- 79 files across the IS-IS tree import `isis/types`, including the wire codec
  (`packet/header.go`, `packet/tlv_core.go`, `packet/checksum.go`). This is the
  downstream wiring proof: the codec compiles against these types and adds no new
  identifier type back here (assumption A-1 confirmed). The whole tree builds on
  darwin and linux.
- Because the identifiers are comparable value arrays, the LSDB and adjacency
  tables in later specs key on them directly with no wrapper struct and no pointer
  identity surprises (A-2 confirmed).
- The reserved-zero-sequence-vs-remaining-lifetime-0-purge split established here
  is the contract isis-6 relies on for origination and purge; getting it wrong at
  the type level would have caused origination loops/flaps downstream.

## Gotchas / Traps
- **Strict canonical grouping in parse.** `parseDottedHex` rejects a string with
  the right number of hex digits but the wrong grouping (e.g. `000100020003` with
  no dots) as `ErrBadGrouping`, because the canonical IS-IS identifier form is
  always dot-grouped. Do not "relax" this into accepting un-dotted hex; the strict
  form is intentional and covered by `TestSystemIDParseRejects` /
  `TestParseRejectsMalformed`.
- **NET split is positional, not delimiter-based.** `NET` accessors slice the
  trailing 7 octets as SystemID+SEL and everything before as AreaID. The text form
  uses the "first octet alone, then 2-octet groups" convention
  (`49.0001.0000.0000.0001.00`), which `appendNETHex` reproduces -- this is a
  DIFFERENT grouping from the plain `appendDottedHex` used by SystemID/LSPID. Two
  separate append helpers exist for exactly this reason; do not unify them.
- **`String()` is not zero-alloc, `AppendTo` is.** `String()` returns an owned
  string so the result copy is one allocation by definition (no-sprintf-alloc
  "Tier 1: one allocation"). Hot paths must call `AppendTo` into a caller buffer.
  The test asserts `AppendTo` == 0 allocs and `String()` <= 1; do not tighten the
  `String()` bound to 0.
- **`MetricFromBytes` cannot range-error** (3 octets can't exceed 24-bit), but
  `NewMetric` can (a uint32 caller can pass > max). The two constructors have
  different error surfaces by design.
- **Bare `go build` is hook-blocked** in this repo (must be `go build -o bin/...`).
  To type-check a library package across GOOS, use `go vet` (it compiles), not
  `go build`. `/tmp` and shell pipes (`| tail`) are also hook-blocked; write logs
  under project `tmp/<subfolder>/` and Read them.

## Deviations from Plan
- The plan's single `TestLifetimeAndHoldingTime` was split into
  `TestRemainingLifetimeBoundaries` + `TestHoldingTimeBoundaries` for clearer
  per-type boundary coverage. No planned test was dropped; several were added
  beyond the plan (`accessors_test.go`, `TestFromBytesRejectsWrongLength`,
  `TestMetricWidthsDistinct`, `TestSequenceNumberWrap`, the two map-key tests,
  `TestAppendDottedHex`, `TestParseDottedHexErrors`).

## Interop validation pending Linux execution
This is a pure leaf type package, so it has no direct interop AC (the Interop
Tests table in the spec is N/A: types carry no wire behaviour alone). Its
correctness is exercised on the wire only through the downstream IS-IS codec and
runtime. The broader IS-IS FRR/QEMU interop scenarios that exchange these encoded
values exist as scenario files (`test/interop/scenarios/isis-*`, `test/isis/*.ci`,
and the QEMU integration tests) but were NOT executed on this darwin host:
on-the-wire interop validation against FRR (adjacency, route convergence,
dual-stack, auth interop) is interop validation pending Linux/QEMU execution. The
code is implemented; only that on-wire validation step remains, and it requires a
Linux host.

## Files
- `internal/plugins/isis/types/systemid.go` (+test): SystemID 6-byte identifier.
- `internal/plugins/isis/types/sourceid.go` (+test): SystemID + pseudonode ID.
- `internal/plugins/isis/types/lspid.go` (+test): SourceID + LSP number,
  big-endian total order for CSNP/PSNP range bounding.
- `internal/plugins/isis/types/net.go` (+test): NET (8..20) and AreaID (1..13),
  positional split, byte-lexicographic ordering, NET dotted-hex convention.
- `internal/plugins/isis/types/metric.go` (+test): Metric (24-bit, TLV 22) and
  PrefixMetric (32-bit, TLV 135/236) as two distinct widths.
- `internal/plugins/isis/types/sequence.go` (+test): SequenceNumber, reserved-0,
  wrap-aware Next/NextChecked.
- `internal/plugins/isis/types/lifetime.go` (+test): RemainingLifetime (IsPurge)
  and HoldingTime, 16-bit seconds.
- `internal/plugins/isis/types/format.go` (+test): shared zero-alloc dotted-hex
  append/parse helpers and the package error set.
- `internal/plugins/isis/types/accessors_test.go`: Equal/Bytes accessor coverage
  (added beyond plan).
- `internal/plugins/isis/types/parse_test.go`: malformed-input rejection across
  all Parse*/FromBytes constructors.
- `internal/plugins/isis/types/doc.go`: leaf-package constraint statement.
