---
kind: directive
level:
stage:
---
**Before any buffer/pool/allocation, trace the full lifecycle: where allocated? who holds it? when copied? when released?**
**Ze lifecycle: allocate at receive (Incoming Peer Pool), share read-only through forwarding, copy only on egress modification (Outgoing Peer Pool), release after TCP write.**
**Acquisition point defines the design: "every dispatch" vs "only on modification" are fundamentally different. A pool is not a counter. Look at filter code + `buildModifiedPayload` to see WHERE modification happens before deciding WHERE buffers come from.**
**Red flags:** new file without checking for similar; function that might duplicate; can't name 3 related files.
