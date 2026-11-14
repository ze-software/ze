# Scenario: ospf-import-policy-bird (OSPF import chain against BIRD 3)

Owner spec: `spec-route-policy-0-umbrella`, AC-9, closed on 2026-11-14 (its
record is in git history). Design: `docs/guide/route-policy.md` and
`docs/architecture/rib/unified-locrib.md`.

## What it proves

An `ospf filter import` chain reads the external route tag BIRD sets and ranks
the route at the distance its `modify` sets:

- BIRD exports 10.88.1.0/24 with `ospf_tag = 100` and 10.88.2.0/24 untagged,
  as AS-external LSAs.
- Ze's chain sets distance 200 on a route tagged 100. Ze also holds a static
  blackhole for both prefixes, at static distance 150.
- 10.88.2.0/24: OSPF at 110 beats static, so `show rib candidates` lists its
  OSPF path at distance 110 and selected.
- 10.88.1.0/24: OSPF at 200 loses to static, so `show rib candidates` lists its
  OSPF path at distance 200 and the static path selected.
- 10.88.3.0/24, a third BIRD external, is rejected by the central `rib` import
  chain: `show rib candidates` lists its OSPF path at distance 110 as hidden by
  the `rib import` chain, `show ospf route` still lists it, and neither
  `show rib` nor the kernel FIB holds it while the untagged 10.88.2.0/24 is
  installed (`checkOSPFImportCentral` in
  `internal/le/interoplab/bgp/check_import_policy_placement.go`).

The two prefixes differ only in the tag, so the tag decides. BIRD is the
reference: it writes the tag into the AS-external LSA (RFC 2328 Section A.4.5),
and Ze must read it from BIRD's encoding.

## Assertions

`checkOSPFImportPolicy` in `internal/le/interoplab/bgp/check_import_policy.go`.
It decodes the JSON of `show rib candidates prefix <p>` and judges each path on
its own (source, distance, selected, hidden), so one path's field can never
stand for another's.

## Status

Runs under the Linux Docker interop harness only.
