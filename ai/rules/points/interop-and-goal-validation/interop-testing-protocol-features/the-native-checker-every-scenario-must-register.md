---
kind: note
level: MUST
stage:
---
Every scenario MUST have an exact-name entry in the owning native checker
registry. BGP uses the package-local `checkers` registry and typed operations in
`internal/le/interoplab/bgp`; IPsec uses `scenarioCheckers` in
`internal/le/interoplab/ipsec`. The checker MUST wait for readiness, assert the
protocol behavior, verify stability where the scenario requires it, and return
an error on failure. `interoplab.Discover` fails closed when a fixture and its
checker registry disagree.
