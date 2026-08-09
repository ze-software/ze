# OSPFv2 authentication

Per-interface authentication for AuType 0 (Null), AuType 1 (Simple password),
AuType 2 (Keyed-MD5 and the RFC 5709 HMAC-SHA family) and AuType 3 (the RFC 7474
extended 64-bit cryptographic sequence).

## Decisions

- **The codec already did the framing, so this layer is cryptography only.** The
  packet checksum excludes the 8-byte auth field, and `Packet.WriteTo` zeroes
  the checksum and sets Packet Length to header plus body for AuType 2 and 3. No
  wire struct changed.
  <!-- source: internal/plugins/ospf/packet/auth_verify.go -- Sign, Verify -->
- **Sign at one transport chokepoint, not per encoder.** A transport signer hook
  authenticates every outgoing packet of all five types in one place. The
  interface, neighbor and LSDB encoders emit AuType 0, and the signer rewrites
  the AuType byte, fixes the checksum, sets the auth field and appends the
  digest.
  <!-- source: internal/plugins/ospf/auth_wiring.go -- signPacket -->
- **Verify at one receive chokepoint.** The dispatcher runs the verify hook after
  the checksum and area checks and before any ISM, NSM or LSDB handler, so a
  failed packet is dropped before protocol processing.
  <!-- source: internal/plugins/ospf/dispatcher.go -- dispatcher -->
- **Go `hmac.New(H, Ko)` matches RFC 5709.** Section 3.3 derives Ko to L octets
  and computes `H(Ko XOR Ipad || msg)` with Ipad of block size B. Ko is shorter
  than B for every SHA in use, so the XOR zero-pads Ko to B, which is what
  standard HMAC does. Keyed-MD5 (RFC 2328) is NOT HMAC: it is
  `MD5(packet || key16)`.
- **A per-chain `extended-sequence` boolean selects AuType 3.** AuType 2 and 3
  share the HMAC-SHA algorithms and differ in the wire AuType, the 64-bit
  sequence trailer and the Section 6 protocol-id key suffix `0x0001`.
  <!-- source: internal/plugins/ospf/auth_keystore.go -- authStore -->

## Traps

- **AuType 3 binds the IP source address into Apad (RFC 7474 Section 5).** The
  first 4 octets of Apad are the IP source address: the interface address on
  send, the packet source on receive. Without it there is no anti-spoof
  protection AND a conformant peer forms zero AuType-3 adjacencies. A
  self-round-trip test cannot catch this, because both sides share the same wrong
  Apad. Sign with one source and verify with another.
- **Replay rejects an EQUAL sequence, not only a lower one.** RFC 7474 Section 2
  requires the received sequence to be strictly greater than the last accepted.
  The send counter increments per packet, so an equal sequence is always a
  duplicate.
- **The replay high-water mark is per OSPF PACKET TYPE**, not per neighbor and
  key-id alone. A single slot drops a legitimately reordered packet of another
  type as a false replay. Exercise an equal sequence AND a second packet type.
- **The boot-count high word is seeded from the wall clock** at key-store
  creation, so the aggregate cryptographic sequence never regresses across a
  restart. A hardcoded zero makes the first packet after a restart lose to the
  neighbour high-water mark, and the adjacency cannot re-form until the peer
  ages out.
- **`$9$` decode falls back to plaintext.** A non-`$9$` value is used raw, so a
  hand-written config before commit-time encoding still works.
- **A sign-then-verify test with one key derivation cannot prove interop.** A
  wrong Section 6 protocol-id suffix self-verifies and fails against another
  implementation.
