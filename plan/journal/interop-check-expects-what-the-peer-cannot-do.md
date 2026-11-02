# An interop check expects what the peer cannot do

The check was written from the RFC or from Ze's side, not from the peer's
source, and asks the peer for a feature it lacks or an output it never prints.
The scenario can never pass, and its red reads as a Ze defect.

Captured: 2026-11-02 rule interop-and-goal-validation/directives/read-the-peer-s-source-before-an-interop-expectation

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-02 | - | `ospfv3-ri-frr` checker | It asked FRR 10.7.1 ospf6d for Router Information support (`router-info`, `database router-information`), which ospf6d does not have: `% Unknown command` | fixed: e206a51fbb reads Ze's RI LSAs in FRR's database as unknown types 0xa00c and 0xc00c |
| 2026-11-02 | - | `ospfv3-debug-inject-frr` checker | It injected a 4-octet Intra-Area-Prefix body that `ospf6_lsa_examin` drops as undersized, and waited for `0x2009`, which FRR never prints for a type it knows | fixed: e206a51fbb injects a valid one-prefix body and waits for the prefix, then for its withdrawal |
