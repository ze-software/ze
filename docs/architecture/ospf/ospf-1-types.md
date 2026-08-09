# OSPFv2 leaf value types

`internal/plugins/ospf/types` holds the OSPFv2 value types that every higher
OSPF package imports. The package has no runtime dependency.

## Decisions

- **Identifiers are fixed 4-byte values.** `RouterID`, `AreaID` and
  `LinkStateID` are `[4]byte` with parse, bytes, `WriteTo`, `AppendTo` and
  `String` methods. They are comparable and safe as map keys. `net.IP` was
  rejected: a slice carries identity bugs that a value type cannot have.
  <!-- source: internal/plugins/ospf/types/routerid.go -- RouterID -->
  <!-- source: internal/plugins/ospf/types/areaid.go -- AreaID -->
- **LSA identity is separate from LSA freshness.** `LSAKey` holds the LS Type,
  the Link State ID and the Advertising Router. Sequence, age and checksum stay
  outside equality and feed the LSDB freshness rules instead.
  <!-- source: internal/plugins/ospf/types/lsakey.go -- LSAKey -->
- **OSPF owns its checksum algorithms.** The package implements Fletcher-16 over
  the OSPF LSA covered window and the RFC 1071 packet checksum. It does not
  import the IS-IS package, although IS-IS uses the same Fletcher family.
  <!-- source: internal/plugins/ospf/types/checksum.go -- FletcherChecksum, InternetChecksumPair -->
- **The checksum functions take covered windows, not whole packets.**
  `FletcherChecksum` takes `lsa[2:]`, which starts at Options and excludes LS
  Age. Callers cannot disagree about the offset convention.
- **`InternetChecksumPair` takes two segments.** The OSPF packet checksum
  excludes the authentication field at bytes 16..23. A two-segment helper keeps
  RFC 1071 ownership in the leaf package and lets the codec avoid a temporary
  concatenated buffer.
- **Formatting follows the IS-IS discipline.** Hot paths use `AppendTo` and
  allocate nothing. `String` returns an owned string and can allocate for that
  result.
  <!-- source: internal/plugins/ospf/types/format.go -- fixed4AppendTo, fixed4String -->

## Constraints on callers

- Higher OSPF packages import these types. Do not redeclare 4-byte identifiers,
  LSA keys, sequence comparison, age masks, metrics, options or checksum
  helpers.
- OSPFv3 does not share this package. Its leaf types live in
  `internal/plugins/ospf/v3/types` with separate wire semantics.
