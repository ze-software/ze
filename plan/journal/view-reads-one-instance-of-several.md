# A view reads one instance of several

A protocol runs several instances of one engine (an address family, an
Instance ID, a VRF), each with its own neighbors, database and routes. The
listing command counts them all, but every other view asks for one instance by
a fixed key and answers only that one. The other instances run with no view of
their state, and the empty answer for a busy instance reads as "nothing here".

The tell is a fixed instance key at the lookup in a command handler, beside a
listing that iterates the set.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-02 | - (found fixing ospf-multiaf-v4-frr) | OSPFv3 views (`internal/plugins/ospf/register.go`, `v6DatabaseDetail`, `riDatabaseSnapshot`) | Every `show ospf ipv6` view but the AF listings calls `v6set.engineFor(afIPv6Unicast)`, and no `show ospf ipv6 route` exists: an ipv4-unicast instance (Instance ID 64) shows no neighbor, LSDB or route, so the interop checker fell back to the peer's kernel route | open: every view answers each running AF with tagged rows and an `address-family` keyword |
| 2026-11-02 | - (found fixing ospf-multiaf-v4-frr) | OSPFv2 views (`internal/plugins/ospf/register.go`) | Every OSPFv2 view but `show ospf instance` reads `eng`, the base engine, so an RFC 6549 instance with a non-zero Instance ID has no neighbor, LSDB or route view | open: the same per-instance answer for the OSPFv2 views |
