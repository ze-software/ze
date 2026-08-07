---
kind: directive
level:
stage:
---
- **Peer Pools** (64 buffers per peer, negotiated size): each peer has an Incoming Peer Pool (inbound) and an Outgoing Peer Pool (outbound modification). Same Peer Pool type, size at init.
- **Global Shared Pool**: byte-budgeted overflow, mixed 4K/64K blocks. Auto-sized from peer prefix maximums via `overflowPoolBudget()`. Pool exhaustion is the backpressure signal.
