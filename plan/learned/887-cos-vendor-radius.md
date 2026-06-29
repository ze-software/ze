# 887 -- Vendor-Specific RADIUS Attributes for CoS/Rate

## Context

BNG operators migrating from Juniper, Cisco, Nokia, Huawei, or MikroTik already have RADIUS servers configured with vendor-specific attributes (VSAs) for QoS profile assignment. Ze's existing "cos:" Filter-Id prefix is the native mechanism, but requiring operators to reconfigure their RADIUS infrastructure blocks migration. Parsing 5 vendors' VSAs as a fallback (Ze prefix wins) enables zero-config migration.

## Decisions

- Chose hardcoded 5-vendor switch over extensible registry because the vendor set is known and bounded; a registry is premature abstraction for a switch statement.
- Chose single extract_vsa.go over per-vendor files because each parser is 5-10 lines; separate files add navigation cost without isolation benefit.
- Chose Ze "cos:" Filter-Id as highest priority over vendor VSA because operator explicitly chose Ze convention; vendor VSA is the migration fallback.
- Chose narrow Cisco-AVPair key matching ("subscriber:sub-qos-policy-{in,out}") over broad "qos" substring match to avoid false positives on unrelated Cisco-AVPair keys.
- Chose MikroTik rate as FilterID-compatible string ("Nbit") over raw uint64 storage because downstream shaper consumes FilterID through traffic.ParseRateBps; converting to "Nbit" format keeps the existing pipeline.
- Chose `math.MaxUint64/mult` overflow guard over unchecked multiply because parseMikrotikRateValue takes untrusted RADIUS input; silent overflow would produce a nonsensical rate.

## Consequences

- Adding a new vendor requires: one constant pair in dict.go, one case in matchVendorCoS or extractVSARate, one test. No registration, no config, no schema.
- MikroTik rate stored as "Nbit" string in FilterID means the shaper sees it as a plain rate, not as a MikroTik-specific value. Upload rate from MikroTik is currently discarded (download only, like existing Filter-Id rate path).
- CoA path has VSA fallback parity with Access-Accept path; both extractCoSProfile and extractRate check VSAs after Filter-Id.

## Gotchas

- EncodeVSA with empty value (0 bytes) produces valid wire bytes; DecodeVSA returns empty value slice. Must check `len(value) == 0` before passing to vendor parsers.
- MikroTik Mikrotik-Rate-Limit format is "rx/tx burst/burst threshold/threshold time/time priority min/min"; must split on space first to isolate the rate pair from burst fields.
- Cisco-AVPair is a general-purpose key=value attribute. Only specific QoS keys should extract CoS profiles; unrecognized keys must be silently ignored (with debug log) to avoid false-positive profile assignment.

## Files

- `internal/component/radius/dict.go` -- vendor ID + attr type constants
- `internal/component/l2tp/plugins/auth_radius/extract_vsa.go` -- vendor parsers
- `internal/component/l2tp/plugins/auth_radius/extract_vsa_test.go` -- 25 unit tests
- `internal/component/l2tp/plugins/auth_radius/extract.go` -- VSA wiring in extractAuthMetadata
- `internal/component/l2tp/plugins/auth_radius/coa.go` -- VSA wiring in extractCoSProfile/extractRate
- `test/plugin/cos-vendor-cisco.ci` -- functional test
- `test/plugin/cos-vendor-coexist.ci` -- functional test
- `docs/comparison.md` -- Vendor-Specific attribute row
- `docs/features.md` -- vendor VSA mention
