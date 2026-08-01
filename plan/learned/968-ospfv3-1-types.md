# 968 - OSPFv3 leaf types package (spec-ospfv3-1)

## Context

First implementation child of the OSPFv3 (RFC 5340) follow-up umbrella. Created the leaf
value-type package `internal/plugins/ospfv3/types/` that every later OSPFv3 child spec
(packet codec, transport, LSDB, SPF, auth, CLI, interop) shares: Router/Area/Link-State
IDs, Instance ID, Interface ID, the 16-bit LS Type with embedded flooding scope, the
comparable LSA key, sequence numbers, ages, the 24-bit Options field, IPv6 prefix
length/options, and metrics. Pure value code; no runtime dependencies.

## Decisions

- **A SEPARATE copy of the OSPFv2 leaf conventions, not a shared package.** The umbrella is
  emphatic (guide §15): do NOT unify v2/v3 (FRR ships two daemons; the LSA registries and
  wire encodings differ enough that sharing leaks detail). So `format.go` re-implements the
  dotted-quad parse / `fixed4` generics / `writeUint*` helpers rather than importing
  `internal/plugins/ospf/types`. A `go/parser` import-guard test (`imports_test.go`)
  enforces that the package imports no `ospf`, `ospfv3` sibling, `component`, or `rib`.
- **Scope lives ON the LS Type, never re-derived.** OSPFv3 widens the LS Type to 16 bits and
  embeds the flooding scope in the top bits: U-bit `0x8000`, S2/S1 scope `0x6000` (shift
  13), 13-bit function code `0x1fff`. `LSType.Scope()`/`FunctionCode()`/`UBit()` decode it
  so the LSDB and flooding (later specs) never carry an ad-hoc scope table (R-2).
- **`LSAKey` is `(LSType, LinkStateID, AdvertisingRouter)` with NO separate scope field** --
  the LS Type already carries scope. Age/sequence/checksum/length are not identity.
- **Width fidelity matters for FRR interop.** OSPFv3 Options is 24 bits (not the OSPFv2 8),
  Metric is 24 bits, the IPv6 prefix byte length is the RFC 5340 padded word rule
  `((PrefixLength+31)/32)*4`, and padding bits past the prefix length must be zero
  (`ValidatePadding`) -- boundary-tested at /0,/1,/31,/32,/33,/64,/127,/128 (R-3).

## Gotchas

- **`uint32(<negative typed constant>)` overflows at COMPILE time.** `InitialSequenceNumber`
  is `LSSequenceNumber(-0x7fffffff)`; `uint32(InitialSequenceNumber)` in a test format
  string is a constant-overflow compile error. Use `int32(...)` + `%d`, or a runtime
  variable. (Runtime int32->uint32 conversion is fine; the constant conversion is not.)
- **AreaID parses BOTH dotted-quad and a plain integer** (`0` == `0.0.0.0`, `16` ==
  `0.0.0.16`): try `parseFixed4` first, fall back to `parseUint32Decimal`.
- **gocritic `dupArg`** flags `x.Compare(x)` (same receiver+arg) in a reflexivity test --
  use a distinct copy variable.

## Verification anchors

- `TestOSPFv3IdentifierParseFormat`, `TestOSPFv3InstanceIDBoundaries`,
  `TestOSPFv3InterfaceIDBoundaries`, `TestOSPFv3LSTypeScopeFunction`/`KnownLSATypes`/
  `ReservedScope`/`LSAKeyComparable`, `TestOSPFv3SequenceBoundaries`,
  `TestOSPFv3AgeBoundaries`, `TestOSPFv3Options24BitRoundTrip`,
  `TestOSPFv3PrefixEncodingBoundaries`, `TestOSPFv3MetricBoundaries`,
  `TestOSPFv3TypesWriteTo`, `TestOSPFv3TypesNoRuntimeImports`. All 10 ACs covered.
- `go test -race` clean; `make ze-lint-changed` 0; `go vet` 0; `make ze-tier-check` 0.
- Next OSPFv3 target (umbrella): `spec-ospfv3-2-wire.md` (packet + LSA codec), which imports
  these types.

## Files

None recorded.
