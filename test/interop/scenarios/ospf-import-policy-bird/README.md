# Scenario: ospf-import-policy-bird (OSPF import chain against BIRD 3)

Owner spec: `plan/post-release/features/spec-route-policy-0-umbrella.md`, AC-9.

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
