---
kind: note
level: MUST
stage:
---
Payloads that cross plugin or component boundaries MUST be self-contained value types.
No pointer fields pointing to data owned by another plugin or component, even when the
target lives in a shared core package.
