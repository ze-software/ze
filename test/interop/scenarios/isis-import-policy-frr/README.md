# Scenario: isis-import-policy-frr (IS-IS import chain between two FRR isisd)

Owner spec: `plan/post-release/features/spec-route-policy-0-umbrella.md`, AC-4.

## What it proves

An `isis filter import` chain decides what IS-IS installs, and nothing else:

- FRR-A advertises 10.77.1.0/24 and 10.77.2.0/24 from its loopback.
- Ze's chain rejects 10.77.1.0/24. `show rib` does not list it, the kernel FIB
  holds no route for it, and `show rib candidates prefix 10.77.1.0/24` lists
  its one IS-IS path hidden, hidden by the `source` chain `isis import`.
- Ze's LSDB still holds the prefix (the chain runs at the Loc-RIB gate, after
  SPF, and never touches the database Ze floods from), and FRR-B, whose only
  neighbor is Ze, installs an IS-IS route for it through Ze. That is the proof
  that Ze floods on the LSP whose prefix it hid from its own RIB.
- 10.77.2.0/24 passes the same chain, so it is in `show rib`, the kernel FIB
  and FRR-B's RIB. Every absence above is read beside that presence, so a dead
  adjacency or an empty RIB cannot pass it.

## Topology

```
  FRR-A (frr)                     Ze                     FRR-B (frr-sink)
  10.77.1.1/24, 10.77.2.1/24      isis filter import     originates nothing
  eth1 10.101.0.3 ---- VLAN 101 ---- eth1 10.101.0.2
                                     eth2 10.102.0.2 ---- VLAN 102 ---- eth1 10.102.0.15
```

The two links are declared in `scenarioLinks`
(`internal/le/interoplab/bgp/register_import_policy.go`) and built by the
harness before each daemon starts. The FRRs share no link, so FRR-B cannot
learn FRR-A's prefixes except through Ze.

## Assertions

`checkISISImportPolicy` in `internal/le/interoplab/bgp/check_import_policy.go`.

## Status

Runs under the Linux Docker interop harness only (raw L2 and FRR isisd).
