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
- **The accept-lifetime gate runs BEFORE the digest and before the replay
  bookkeeping.** RFC 7474 Section 4 requires the accept window to include the
  current time for a key used on reception, so a key outside its window is
  skipped and can neither accept the packet nor record its sequence number. A
  gate placed after the digest comparison would let an out-of-window key advance
  the high-water mark, and the packet the operator meant to refuse would then
  block the legitimate one behind it. The send side and the receive side point
  the same way but do the opposite thing: `selectSendKey` keeps signing with an
  expired key (signing stale beats sending unauthenticated), and `verify` refuses
  every key whose window has closed (refusing stale beats authenticating a
  neighbor the operator retired).
  <!-- source: internal/plugins/ospf/auth_keystore.go -- resolvedKey.acceptsAt, authStore.verify -->
- **Replay rejects an EQUAL sequence, not only a lower one.** RFC 7474 Section 2
  requires the received sequence to be strictly greater than the last accepted.
  The send counter increments per packet, so an equal sequence is always a
  duplicate.
- **The replay high-water mark is per OSPF PACKET TYPE**, not per neighbor and
  key-id alone. A single slot drops a legitimately reordered packet of another
  type as a false replay. Exercise an equal sequence AND a second packet type.
- **The boot-count high word comes from the daemon's persistent store.** After
  its handshake, each runtime engine requests an atomic `state-increment` before
  interface subscriptions or packet processing. The daemon holds its write guard
  until the increment is durable. Reload-created engines follow the same path;
  they do no state I/O during construction or configure. Unavailable storage,
  corrupt counters and uint32 exhaustion refuse startup, since a clock-derived
  value cannot guarantee the order RFC 7474 Section 2 requires.
  <!-- source: internal/plugins/ospf/state.go -- engine.initializeState -->
  <!-- source: internal/core/statestore/statestore.go -- Increment -->
  Low-word wrap requests another durable increment through the same runtime
  client. Neither counter advances in memory until that reservation succeeds;
  unavailable storage, exhausted counters and non-increasing returned values
  refuse signing. The transport drops the refused packet, including routed
  virtual-link sends, rather than sending the original unauthenticated payload.
  <!-- source: internal/plugins/ospf/auth_keystore.go -- authStore.signKey -->
  <!-- source: internal/plugins/ospf/transport/transport.go -- SendPacket, SendPacketRouted -->
- **Storage loss and router replacement require new authentication keys.** RFC
  7474 Section 8 requires the shared authentication keys to change if repair or
  upgrade loses the non-volatile contents, or if the OSPFv2 router is replaced.
  Before the affected router resumes OSPF, its peers must use the replacement
  secrets and stop accepting the old ones. Changing only Key IDs does not change
  the key material, and retaining old keys in an open accept window still permits
  packets authenticated with those secrets.
  The daemon starts an absent boot counter at one, as it does on first use.
  Successful creation of that counter cannot distinguish a new deployment from
  lost storage. Startup refuses reported storage errors; it neither detects every
  storage-loss/replacement event nor rotates shared secrets on the peers.
  <!-- source: internal/core/statestore/statestore.go -- Increment -->
  <!-- source: internal/plugins/ospf/auth_keystore.go -- loadOSPFBootCount, resolvedKey.acceptsAt -->
- **`$9$` decode falls back to plaintext.** A non-`$9$` value is used raw, so a
  hand-written config before commit-time encoding still works.
- **Independent digest construction is needed for interop evidence.** Sign and
  Verify share Ko derivation and Apad construction, so matching errors can pass a
  round trip. SHA tests construct Ko, Ipad, Opad and Apad independently; the
  AuType-3 padding tests also supply the source address and protocol-ID suffix.
  <!-- source: internal/plugins/ospf/packet/auth_verify_test.go -- rfc5709ReferenceDigest, TestRFC5709ReceiveIndependentDigest, TestRFC5709ReceiveRejectsWrongHashConstruction -->
  <!-- source: internal/plugins/ospf/packet/auth_rfc7474_test.go -- TestRFC7474KoZeroPaddedToBlockSize, TestRFC7474KoNonZeroPadRejected -->
