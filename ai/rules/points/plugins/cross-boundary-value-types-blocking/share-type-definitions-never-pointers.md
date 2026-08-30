---
kind: directive
level: MAY
stage:
---
- **A shared type definition such as `family.Family` or `RouteChangeBatch` MAY cross a boundary, because it is a compile-time contract rather than data.** The ban is on one plugin holding a pointer to data another plugin allocated.
