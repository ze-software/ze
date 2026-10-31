# Scenario: isis-export-policy-frr (IS-IS export chain toward FRR isisd)

Owner spec: `plan/post-release/features/spec-route-policy-0-umbrella.md`, AC-12.

## What it proves

An `isis filter export` chain decides which redistributed routes enter Ze's
own LSP, and at what metric, as FRR sees it:

- Ze redistributes three static routes into IS-IS: 10.66.0.1/32 (tag 300),
  10.66.0.2/32 and 10.66.1.0/24.
- The chain is `filters [ loopbacks tagged-loopback-metric ]; final accept;`.
  `loopbacks` is a prefix-list that accepts only /32s, so it rejects the /24.
  `tagged-loopback-metric` is a modify that matches tag 300 and sets metric 77.
- FRR installs 10.66.0.1/32 through Ze at metric 87, its own link metric to Ze
  (10) plus the 77 Ze advertised, and 10.66.0.2/32 at 110, the link metric plus
  Ze's default redistribution metric 100. The two values differ, so a metric
  set on every route, or on none, fails.
- FRR holds no route for 10.66.1.0/24. That absence is read in the same
  `show ip route isis json` answer that holds both /32s, which rode in the same
  LSP, so a dead adjacency or an empty table cannot pass it.

## Topology

```
  FRR isisd  <--- IS-IS p2p over eth0 (shared bridge) --->  Ze
   isis metric 10                                         static -> isis
                                                          isis filter export
```

## Assertions

`checkISISExportPolicy` in `internal/le/interoplab/bgp/check_export_policy.go`,
deciding through `isisExportPolicyVerdict` in `check_export_policy_predicate.go`:

1. FRR's `show isis neighbor` shows the adjacency Up.
2. One `show ip route isis json` answer holds 10.66.0.1/32 selected through
   Ze's eth0 address at metric 87, 10.66.0.2/32 the same way at metric 110, and
   no 10.66.1.0/24.

## Red walk

Break `exportVerdict` (`internal/plugins/isis/redistribute/export.go`) so it
answers accepted at `DefaultRedistMetric` whatever the chain says. Assertion 2
must then fail twice over: the /24 is installed and 10.66.0.1/32 sits at 110.

## Status

Runs under the Linux Docker interop harness only (raw L2 and FRR isisd). Not
yet run: the config validates (`ze config validate`), and the checker's
predicates are driven in both polarities by `TestBespokeCheckerBranches`.
