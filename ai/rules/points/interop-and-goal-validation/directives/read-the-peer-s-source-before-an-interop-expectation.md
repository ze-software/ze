---
kind: directive
level: MUST
stage:
---
**Before an interop check expects a peer daemon to accept, store, print or send something, you MUST read that behavior in the peer's source at the version the lab runs, and name the function.** A check written from the RFC or from Ze's side asserts what the peer ought to do. When the peer cannot do it (a feature it lacks, a size check that drops the body, an output that never prints the expected token), the scenario can never pass, and its red reads as a Ze defect.
