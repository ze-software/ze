---
kind: fence
level:
stage:
---
```
TCP recv → readBufPool (4K/64K)
    → WireUpdate (lazy, references readBuf)
    → attribute extraction (lazy iterators, no copy)
    → pool dedup (per-attribute-type, refcounted)
    → RIB entry (NLRI → attribute Handle refs)
    → route selection (operates on Handles)
    → outbound building (WriteTo into peerPool buffer)
    → TCP send → release peerPool buffer
```
