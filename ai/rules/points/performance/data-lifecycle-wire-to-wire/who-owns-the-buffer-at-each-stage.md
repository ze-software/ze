---
kind: table
level:
stage:
---
| Stage | Buffer owner | What happens |
|---|---|---|
| TCP read | Incoming Peer Pool (ring of 64 buffers) | Wire bytes land in pool buffer |
| Parsing | Same buffer | WireUpdate holds byte slice into pool buffer, no copy |
| Attribute extract | Same buffer | Lazy iterators return sub-slices, no copy |
| Pool dedup | Attribute pool | First time: copy into pool slab. Subsequent: refcount++ |
| Forwarding (same context) | Same pool buffer | Zero-copy: ContextID match means wire bytes are reusable |
| Forwarding (different context) | Outgoing Peer Pool | Copy-on-modify: build modified UPDATE into outgoing buffer |
| Overflow | Global Shared Pool (MixedBufMux) | Byte-budgeted, mixed 4K/64K blocks |
