# Vendor-specific RADIUS attributes for class of service and rate

An operator migrating from Juniper, Cisco, Nokia, Huawei or MikroTik already has
a RADIUS server that assigns quality-of-service profiles through
vendor-specific attributes. Ze's native mechanism is the `cos:` prefix on
Filter-Id. Parsing those five vendors' attributes as a fallback means the
operator does not have to reconfigure the RADIUS infrastructure to migrate.

<!-- source: internal/component/l2tp/plugins/authradius/extract_vsa.go -- extractVSACoSProfile, extractVSARate, matchVendorCoS, parseCiscoAVPairCoS, parseMikrotikRate -->
<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- extractAuthMetadata -->

## Decisions

**A hardcoded five-vendor switch, not an extensible registry.** The vendor set
is known and bounded. A registry is premature abstraction over a switch
statement.

**One file, not a file per vendor.** Each parser is five to ten lines.
Separate files add navigation cost and isolate nothing.

**The Ze `cos:` prefix wins over any vendor attribute.** An operator who spells
the Ze convention has chosen it. The vendor attribute is the migration fallback,
so it is consulted second.

**Cisco AVPair matching is narrow.** Only the two subscriber quality-of-service
policy keys match. A broad substring match on "qos" produces false positives on
unrelated AVPair keys.

**A MikroTik rate is stored as a Filter-Id-compatible string.** The downstream
shaper parses the Filter-Id through the rate parser, so converting to the same
spelling keeps one pipeline instead of two.

**The MikroTik rate multiply is overflow-guarded.** The input is untrusted
RADIUS data, and a silent overflow produces a nonsensical rate.

## Consequences worth knowing

- Adding a vendor costs one constant pair in the RADIUS dictionary, one case in
  the matcher or the rate extractor, and one test. No registration, no config,
  no schema.
- The MikroTik upload rate is discarded. Only the download rate reaches the
  shaper, which matches the existing Filter-Id rate path.
- The change-of-authorization path has the same fallback as the Access-Accept
  path. Both check the vendor attributes after Filter-Id.

<!-- source: internal/component/radius/dict.go -- vendor id and attribute type constants -->

## Traps this code exists to avoid

**An empty vendor attribute value is valid on the wire.** Encoding a
vendor-specific attribute with a zero-length value produces valid bytes, and
decoding returns an empty slice. Check the length before handing it to a vendor
parser.

**The MikroTik rate-limit value is not just a rate pair.** The format is
`rx/tx burst/burst threshold/threshold time/time priority min/min`. Split on the
space first, or the burst fields are parsed as the rate.

**An unrecognized Cisco AVPair key must be ignored silently.** The attribute is
a general-purpose key and value carrier. Assigning a profile from an
unrecognized key is a false positive on a subscriber's service.
