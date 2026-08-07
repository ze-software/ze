---
kind: directive
level:
stage:
---
**A guard must fail closed or say something. Silent degradation into a permissive no-op is the bug, and a zero value that downstream reads as a legitimate answer is how it hides.**
