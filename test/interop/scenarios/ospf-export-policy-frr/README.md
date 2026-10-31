# Scenario: ospf-export-policy-frr (OSPFv2 export chain toward FRR ospfd)

Owner spec: `plan/post-release/features/spec-route-policy-0-umbrella.md`, AC-13.

## What it proves

An `ospf filter export` chain decides which redistributed routes Ze originates
as AS-External-LSAs, and what each LSA carries, as FRR sees it:

- Ze redistributes three static routes into OSPF: 10.67.1.0/24 (tag 100),
  10.67.2.0/24 and 10.67.3.0/24.
- The chain is `filters [ ospf-export-refused ospf-export-tagged ]; final accept;`.
  `ospf-export-refused` is a match that rejects 10.67.3.0/24.
  `ospf-export-tagged` is a modify that matches protocol static and tag 100 and
  sets metric 50, metric-type type-1 and tag 200.
- FRR holds Ze's LSA for 10.67.1.0/24 at metric 50, type E1, External Route
  Tag 200: every value the modify set, and the tag differs from the route's own
  100, so the set tag is what reached the wire.
- FRR holds Ze's LSA for 10.67.2.0/24 at the defaults: metric 20, E2, tag 0.
  A modify applied to every route fails here.
- FRR holds no LSA for 10.67.3.0/24, read in the same
  `show ip ospf database external json` answer that holds the other two.

Ze's router ID is 10.255.0.2, outside the lab subnet, so the runner's address
rewrite leaves it as written and the checker binds each LSA to Ze as its
advertising router. The scenario covers OSPFv2 only; no OSPFv3 scenario
reads an export chain's sets yet.

## Topology

```
  FRR ospfd  <--- OSPFv2 p2p over eth0 (shared bridge), area 0 --->  Ze
                                                                   static -> ospf
                                                                   ospf filter export
```

## Assertions

`checkOSPFExportPolicy` in `internal/le/interoplab/bgp/check_export_policy.go`,
deciding through `ospfExportPolicyVerdict` in `check_export_policy_predicate.go`:

1. FRR's `show ip ospf neighbor` shows the adjacency Full.
2. One `show ip ospf database external json` answer holds exactly one LSA from
   10.255.0.2 for 10.67.1.0 (mask /24, metric 50, E1, tag 200), exactly one for
   10.67.2.0 (mask /24, metric 20, E2, tag 0), and none for 10.67.3.0.

The checker reads FRR's keys `linkStateId`, `advertisingRouter`,
`networkMask`, `metricType`, `metric` and `externalRouteTag`, wherever FRR
nests the LSA objects. The key names are FRR's `show_as_external_lsa_detail`
(`ospfd/ospf_vty.c`) as remembered, not yet read from a run against the lab
image: the first Linux run confirms them.

## Red walk

Break `applyExportSets` (`internal/plugins/ospf/redistribute/export.go`) so it
returns the External it was given. Assertion 2 must then fail: the LSA for
10.67.1.0/24 carries metric 20, E2 and tag 100.

## Status

Runs under the Linux Docker interop harness only (FRR ospfd over the shared
bridge). Not yet run: the config validates (`ze config validate`), and the
checker's predicates are driven in both polarities by
`TestBespokeCheckerBranches`.
