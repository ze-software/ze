# 955 -- OSPFv2 types: keep value leaves small and protocol-owned

## Context

`spec-ospf-1-types.md` introduced the OSPFv2 leaf package under
`internal/plugins/ospf/types`. Thomas asked that OSPF follow the structure of the
existing IS-IS code where it makes sense. The closest model was
`internal/plugins/isis/types`, but OSPF needed its own identifiers, checksum
coverage rules, and metric semantics.

## Decisions

- **Use fixed value types for identifiers.** `RouterID`, `AreaID`, and
  `LinkStateID` are `[4]byte` values with parse, bytes, `WriteTo`, `AppendTo`,
  and `String` methods. They are comparable, map-key safe, and avoid `net.IP`
  slice identity bugs.
- **Keep LSA identity separate from LSA freshness.** `LSAKey` is only `(LS Type,
  Link State ID, Advertising Router)`. Sequence, age, and checksum stay outside
  equality and feed later LSDB freshness logic.
- **Own OSPF checksum algorithms in OSPF.** The package implements Fletcher-16
  over the OSPF LSA covered window and RFC 1071 packet checksum. It does not
  import IS-IS, even though IS-IS uses the same Fletcher family.
- **Accept covered windows, not whole packets.** `FletcherChecksum` takes
  `lsa[2:]`, starting at Options and excluding LS Age. This avoids callers
  disagreeing about offset conventions.
- **Add a two-segment Internet checksum helper.** OSPF packet checksum excludes
  auth bytes 16..23. `InternetChecksumPair` keeps RFC 1071 ownership in the leaf
  package while letting the packet codec avoid a temporary concatenated buffer.
- **Follow IS-IS formatting discipline.** Hot paths use zero-allocation
  `AppendTo`; `String` returns an owned string and may allocate for that result.

## Consequences

- Higher OSPF packages should import these value types instead of redeclaring
  4-byte identifiers, LSA keys, sequence comparison, age masks, metrics, options,
  or checksum helpers.
- `make ze-validate` will keep reporting some exported type APIs until config,
  LSDB, SPF, and runtime children consume them. This is expected during the
  back-to-back OSPF implementation, not a reason to add shims.
- Any future OSPFv3 implementation must not share this package. OSPFv3 gets its
  own `internal/plugins/ospfv3/types` package with separate wire semantics.

## Files

None recorded.
