---
kind: note
level:
stage:
---
Shared type definitions (`family.Family`, `RouteChangeBatch`) are fine as compile-time contracts.
What is forbidden is one plugin holding a pointer to data another plugin allocated.
