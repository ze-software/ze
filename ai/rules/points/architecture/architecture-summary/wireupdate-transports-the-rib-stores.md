---
kind: directive
level:
stage:
---
- WireUpdate = transport (lazy parse via iterators, keeps wire refs)
- RIB = storage (NLRI → attribute refs into per-type pools, NOT WireUpdate)
- Per-attribute-type pools with dedup. Per-family NLRI pools.
