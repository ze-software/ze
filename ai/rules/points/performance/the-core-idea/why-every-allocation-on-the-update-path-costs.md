---
kind: note
level:
stage:
---
Ze processes millions of BGP UPDATEs per second. Each UPDATE touches: wire
parsing, attribute extraction, pool dedup, RIB storage, route selection,
filter evaluation, UPDATE building, and TCP write. Every allocation on
this path adds GC pressure and latency. Ze eliminates allocations through
three interlocking strategies:
