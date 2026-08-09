# OSPFv2 multi-instance

RFC 6549 splits the former 2-byte OSPFv2 AuType field into a 1-byte Instance ID
at offset 14 and a 1-byte AuType at offset 15, so several OSPF instances share
an interface and are demultiplexed by Instance ID. Ze runs one full OSPFv2
engine per configured Instance ID.

## Decisions

- **An instance manager owns a map of Instance ID to engine.** Instance 0 is
  always present and additionally owns redistribution and default origination. A
  non-zero instance runs the core link-state protocol on its OWN raw socket, and
  the dispatcher demultiplexes by Instance ID.
  <!-- source: internal/plugins/ospf/multi_instance.go -- instanceManager -->
- **Per-instance transport, not one shared socket with fan-out.** The transport
  signer and the interface up and down hooks are per transport, so a shared
  transport would reintroduce shared mutable state.
- **Config carries a per-interface `instance-id` LEAF-LIST**, not a single leaf
  and not a top-level instance list. It is the only shape that expresses "two
  instances on one interface" (RFC 6549 Section 3.1). An absent leaf-list means
  instance 0.
- **The header split is byte-for-byte identical for instance 0.** Offset 14 was
  the old AuType high byte, and it was always zero because AuType is below 256.
  A golden re-encode test locks this.
  <!-- source: internal/plugins/ospf/packet/header.go -- Header -->

## Constraints on callers

- Every OSPFv2 common header now carries the Instance ID at offset 14. Each
  encoder stamps it, and the dispatcher drops and counts a mismatch before any
  handler runs.
- **The authentication signer must NOT write offset 14.** It writes offset 15
  only. Writing offset 14 clobbers the Instance ID. The digest covers offset 14,
  so the Instance ID is authenticated.
  <!-- source: internal/plugins/ospf/auth_wiring.go -- signPacket -->
- **Opaque consumers and redistribution bind to the base engine only.** The
  opaque registry is a package-global map with duplicate rejection, so a
  per-instance build beyond the base logs a harmless already-registered warning.
  This is a documented single-engine limit and is outside RFC 6549.
