# Wire carries both arms of an either-or

An RFC says "add A, otherwise add B", and the encoder emits A and B together.
Each arm is right on its own, every test reads its own arm, and the peer
receives a record the RFC never allows beside the other.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-21 | - (rfc2328 walk, RFC2328-12.4.1-1) | `internal/plugins/ospf/lsdb/origination.go::routerLinks` | On a broadcast or NBMA interface with an elected DR, the router-LSA carries a Type 2 transit link AND a Type 3 stub link for the same subnet. RFC 2328 Section 12.4.1.1 adds the Type 2 link when the router is adjacent to the DR and "otherwise" the Type 3 stub link, never both | Emit the stub link only on the branches Section 12.4.1.1 names (Waiting state, or no full adjacency with the DR); then correct the tests that pin the pair |
