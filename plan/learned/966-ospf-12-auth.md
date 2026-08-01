# 966 - OSPFv2 authentication (RFC 2328 App D / 5709 / 7474)

## Context

Completed `plan/spec-ospf-12-auth.md`: per-interface OSPFv2 authentication for AuType 0
(Null), AuType 1 (Simple password), AuType 2 (Keyed-MD5 + RFC 5709 HMAC-SHA-1/256/384/
512), and AuType 3 (RFC 7474 extended 64-bit cryptographic sequence). Key chains with
area `inherit` and hitless rotation, `$9$`-encoded secrets, constant-time compare,
per-neighbour anti-replay, and the `ze_ospf_auth_failures_total{interface,reason}`
metric. Sign-on-send and verify-before-routing for all five packet types.

## Decisions

- **The ospf-2 codec already did the hard framing.** `PacketChecksum` excludes the 8-byte
  auth field, and `Packet.WriteTo` zeroes the checksum + sets Packet Length to header+body
  for AuType 2/3 (trap #10). So this spec is purely the cryptographic backend on top: set
  the auth field, compute the digest, append the trailer. No wire-struct change.
- **Sign at one transport chokepoint, not per encoder.** A `transport.SetSigner` hook
  applied inside `SendPacket` authenticates every outgoing packet (all five types) in one
  place. Because the iface/neighbor/lsdb encoders default to AuType 0, `signPacket`
  rewrites the AuType byte, fixes the checksum (recompute for AuType 1 since the AuType
  byte is inside the checksum region; keep zero for crypto), sets the auth field, and
  appends the digest -- avoiding threading auth into every send path.
- **Verify at one RX chokepoint.** A `dispatcher.authOK` hook runs after the checksum and
  area checks and before any ISM/NSM/LSDB handler, so a failed packet is dropped before
  protocol processing.
- **Go's `hmac.New(H, Ko)` matches RFC 5709.** RFC 5709 derives Ko to L octets then does
  `H(Ko XOR Ipad || msg)` with Ipad of block size B; since Ko (L) is shorter than B for
  SHA, the XOR zero-pads Ko to B -- exactly what standard HMAC does. So compute Ko per
  §3.3 step 1 and hand it to `hmac.New`. Keyed-MD5 (RFC 2328) is NOT HMAC: it is
  `MD5(packet || key16)`.
- **A per-chain `extended-sequence` boolean selects AuType 3** (added to ze-ospf-conf.yang),
  since AuType 2 and 3 share the HMAC-SHA algorithms and differ only in the wire AuType +
  the 64-bit sequence trailer + the §6 protocol-id key suffix (0x0001).

## Gotchas

- **Reuse before reinvent (again).** The whole crypto codec scaffolding existed: the
  `AuType` constants, `AuthField` + `CryptoKeyID/CryptoAuthDataLen/CryptoSequence`
  accessors, the auth-field-excluding checksum, and the verify-side AuType-2/3 zero-checksum
  rule were all already shipped by ospf-2/ospf-7. The YANG key-chain schema (chain, key-id,
  algorithm enum, `$9$` secret, lifetimes, per-interface `authentication { mode inherit }`)
  was shipped by ospf-4, and `secret.Encode/Decode` + the RFC 5709/7474 summaries already
  existed. The work was the crypto math + key store + the two hook points.
- **`$9$` decode falls back to plaintext.** `secret.Decode` returns `ErrNoPrefix` for a
  non-`$9$` value; `decodeSecret` uses the raw string then (plaintext config before
  commit-time encoding) so a hand-written test/config still works.
- **Boot-count seeded from wall-clock; exact NVRAM persistence out of scope.** The
  high-order boot word is seeded at `newAuthStore` from `time.Now().Unix()` so the
  aggregate cryptographic sequence never regresses across a restart (R-8); for AuType 2
  the on-wire 32-bit value is `bootCount + counter` (the boot word is otherwise truncated
  away). **Review fix:** the original `bootCount` was hardcoded 0, so after a restart the
  first packet's seq <= the neighbour's high-water and the adjacency could not re-form
  until the peer aged out -- the spec's own Security Properties claimed the seed existed
  but the code omitted it. The per-packet low-order counter still enforces intra-session
  replay (AC-9/10).
- **AuType 3 binds the IP source address into Apad (RFC 7474 §5, review BLOCKER).** The
  original code used the plain RFC 5709 Apad for AuType 3; §5 MUST initialise the first 4
  octets of Apad to the IP source address (send = interface address, receive = packet
  source). Without it there is no anti-spoof protection AND a conformant peer (FRR/Cisco)
  forms zero AuType-3 adjacencies. `Sign`/`Verify` gained a `src [4]byte` param
  (`apadSrc`); TX captures the interface address at `configure` time (`srcByIface`, off the
  per-packet path), RX uses `rp.Src`. Self-round-trip tests cannot catch this (both sides
  share the same wrong Apad) -- `TestOSPFAuthType3SourceBinding` signs with one source and
  verifies with another to prove the binding.
- **Disjoint-prefix-style test blind spot, avoided here.** The crypto tests sign then
  verify with the same key derivation, so a wrong §6 protocol-id suffix would self-verify
  but fail FRR interop -- the AuType-3 key suffix is therefore an interop item (ospf-13),
  not provable by the unit round-trip.
- **Replay must reject EQUAL sequences, not just lower (review-gate BLOCKER).** RFC 7474 §2
  requires the received sequence be STRICTLY greater than the last accepted; an initial
  `seq < last` accepted an exact replay (`seq == last`) because the send counter increments
  per packet, so equal is always a duplicate. Fixed to `seq <= last`. And the high-water
  mark is per OSPF PACKET TYPE, not just per (neighbor, key-id): a single slot drops a
  legitimately reordered lower-sequence packet of another type as a false replay. The
  single-packet-type replay test missed both -- exercise equal-seq AND a second packet type.

## Verification anchors

- `TestOSPFAuthSignVerifySimple` / `TestOSPFAuthSignVerifyCrypto` (5 algos) / `TestOSPFAuthType3SequenceTrailer` (`packet/auth_verify_test.go`) - field layout, appended digest, zero checksum, Packet Length, wrong-key, tamper.
- `TestOSPFAuthKeyStore` / `TestOSPFAuthStoreSignVerify` / `TestOSPFAuthRotation` / `TestOSPFAuthExtendedSequence` / `TestOSPFAuthReplay` (`auth_keystore_test.go`) - inherit, AuType mismatch, rotation overlap, AuType 3 selection, replay reject.
- `TestEngineSignPacketCrypto` / `TestEngineSignPacketSimple` / `TestEngineSignPacketNoAuth` (`auth_wiring_test.go`) - the TX sign hook round-trips through verify.
- `test/ospf/ospf-auth.ci` - config surface (key chain, extended-sequence, inherit, algorithm enum rejection). FRR `ospfd` MD5/HMAC-SHA interop is owned by spec-ospf-13.

## Files

None recorded.
