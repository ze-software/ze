---
kind: directive
level: MUST
stage:
---
- WireUpdate MUST transport data only (lazy parse via iterators, keeps wire refs).
- RIB MUST store NLRI -> attribute refs into per-type pools, and MUST NOT store WireUpdate refs.
- Code MUST use per-attribute-type pools with dedup, and per-family NLRI pools.
