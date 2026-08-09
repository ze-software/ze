# OSPFv3 leaf value types

`internal/plugins/ospf/v3/types` holds the OSPFv3 (RFC 5340) value types: the
identifiers, the 16-bit LS Type with its embedded flooding scope, the LSA key,
sequence numbers, ages, the 24-bit Options field, IPv6 prefix length and
options, and metrics. It is pure value code with no runtime dependency.

## Decisions

- **A separate copy of the OSPFv2 leaf conventions, not a shared package.** The
  LSA registries and the wire encodings differ enough that sharing would leak
  version detail, so the dotted-quad parse, the fixed-4 generics and the write
  helpers are re-implemented here. An import-guard test enforces that the
  package imports no sibling, component or RIB package.
  <!-- source: internal/plugins/ospf/v3/types/format.go -- parseFixed4 -->
- **Scope lives ON the LS Type and is never re-derived.** OSPFv3 widens the LS
  Type to 16 bits: U-bit `0x8000`, S2 and S1 scope bits `0x6000` shifted by 13,
  and a 13-bit function code `0x1fff`. The type decodes its own scope, function
  code and U-bit, so the LSDB and the flooding path carry no ad-hoc scope table.
  <!-- source: internal/plugins/ospf/v3/types/lsa.go -- LSType, Scope, Known -->
- **`LSAKey` is the LS Type, the Link State ID and the Advertising Router, with
  NO separate scope field.** The LS Type already carries the scope. Age,
  sequence, checksum and length are not identity.
- **Field widths follow RFC 5340 exactly, for interop.** Options is 24 bits, not
  the OSPFv2 8. Metric is 24 bits. The IPv6 prefix byte length is the padded
  word rule `((PrefixLength+31)/32)*4`, and the padding bits past the prefix
  length are zero.
  <!-- source: internal/plugins/ospf/v3/types/prefix.go -- PrefixLength, ByteLen, ValidatePadding -->
  <!-- source: internal/plugins/ospf/v3/types/options.go -- Options -->

## Traps

- **`uint32(<negative typed constant>)` overflows at COMPILE time.** The initial
  sequence number is a negative typed constant, so a `uint32` conversion of it
  in a format string does not build. Use `int32` with `%d`, or a runtime
  variable. A runtime int32 to uint32 conversion is fine.
  <!-- source: internal/plugins/ospf/v3/types/sequence.go -- InitialSequenceNumber -->
- **An area id parses BOTH dotted-quad and a plain integer.** `0` equals
  `0.0.0.0` and `16` equals `0.0.0.16`. The parser tries the fixed-4 form first
  and falls back to a decimal.
