---
kind: directive
level: MUST
stage:
---
- **Peer Pools** (64 buffers per peer, negotiated size): each peer has an Incoming Peer Pool (inbound) and an Outgoing Peer Pool (outbound modification). Encoding code MUST take a buffer from the peer pool matching the direction. Both pools use the same Peer Pool type, sized at init.
- **Global Shared Pool**: byte-budgeted overflow, mixed 4K/64K blocks. Auto-sized from peer prefix maximums via `overflowPoolBudget()`. Code MUST treat pool exhaustion as the backpressure signal.
